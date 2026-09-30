# smoke.ps1 — end-to-end smoke test for Dream Interviewer.
#
#   pwsh -File scripts/smoke.ps1
#   pwsh -File scripts/smoke.ps1 -Port 8099 -KeepData
#
# Boots the real server against a throwaway data directory, walks the
# entire product flow over HTTP (onboard → create → start → answer →
# finish → export → stats), and asserts on the responses. It exercises
# the live interviewer model, so it takes a minute or two and needs a
# working LLM configuration; when the model is unavailable it asserts the
# documented offline fallback instead, which is why the interview must
# score above zero either way.
#
# Exit code 0 = all assertions passed, 1 = something is broken.

[CmdletBinding()]
param(
    [int]$Port = 8099,
    [string]$DataDir = "data-smoke",
    [switch]$KeepData,
    # NB: not -Verbose — that is a common parameter CmdletBinding already defines.
    [switch]$ShowFrames
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $root "bin\smoke-server.exe"
$dataPath = Join-Path $root $DataDir

$script:failures = 0
$script:checks = 0
$proc = $null

function Write-Step($msg) { Write-Host "`n=== $msg ===" -ForegroundColor Cyan }

function Assert($condition, $label, $detail) {
    $script:checks++
    if ($condition) {
        Write-Host "  PASS  $label" -ForegroundColor Green
    } else {
        $script:failures++
        Write-Host "  FAIL  $label" -ForegroundColor Red
        if ($detail) { Write-Host "        $detail" -ForegroundColor DarkRed }
    }
}

# Parse an SSE response body into its JSON frames.
function Parse-Frames([string]$body) {
    $frames = @()
    foreach ($line in ($body -split "`n")) {
        $t = $line.Trim()
        if (-not $t.StartsWith("data: ")) { continue }
        try { $frames += ($t.Substring(6) | ConvertFrom-Json) } catch { }
    }
    return $frames
}

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Path,
        $Body,
        [int]$TimeoutSec = 300,
        # Optional second identity, for the RBAC checks. Defaults to the
        # admin session.
        $Session,
        # Authenticate with an API key INSTEAD of a session. Sending both
        # would let a cookie mask a key that has actually lost its rights,
        # which is exactly the bug the demoted-admin check exists to catch.
        [string]$Bearer
    )
    # NB: not `$args` — that is an automatic variable in PowerShell.
    $req = @{
        Uri                = "http://127.0.0.1:$Port$Path"
        Method             = $Method
        TimeoutSec         = $TimeoutSec
        UseBasicParsing    = $true
        # 4xx is an EXPECTED outcome for the guardrail checks; the
        # assertions branch on the `ok` envelope, not the HTTP status.
        SkipHttpErrorCheck = $true
    }
    if ($Bearer) {
        $req.Headers = @{ Authorization = "Bearer $Bearer" }
    } elseif ($Session) {
        $req.WebSession = $Session
    } else {
        $req.WebSession = $script:session
    }
    if ($null -ne $Body) {
        $req.Body = ($Body | ConvertTo-Json -Depth 8 -Compress)
        $req.ContentType = "application/json"
    }
    $res = Invoke-WebRequest @req
    return $res.Content
}

function Invoke-Json {
    param([string]$Method, [string]$Path, $Body, [int]$TimeoutSec = 300, $Session, [string]$Bearer)
    $raw = Invoke-Api -Method $Method -Path $Path -Body $Body -TimeoutSec $TimeoutSec -Session $Session -Bearer $Bearer
    return $raw | ConvertFrom-Json
}

try {
    # --- build --------------------------------------------------------------
    Write-Step "build server"
    & go build -o $binary ./cmd/server
    Assert ($LASTEXITCODE -eq 0) "server builds" "go build exited $LASTEXITCODE"

    # --- start --------------------------------------------------------------
    Write-Step "start server on :$Port (data: $DataDir)"
    if (Test-Path $dataPath) { Remove-Item -Recurse -Force $dataPath }
    New-Item -ItemType Directory -Force -Path $dataPath | Out-Null

    $proc = Start-Process -FilePath $binary `
        -ArgumentList @("-port", "$Port", "-data-dir", $dataPath) `
        -PassThru -NoNewWindow -RedirectStandardOutput (Join-Path $dataPath "server.log") `
        -RedirectStandardError (Join-Path $dataPath "server.err.log")

    $ready = $false
    foreach ($i in 1..60) {
        Start-Sleep -Milliseconds 400
        try {
            $null = Invoke-WebRequest "http://127.0.0.1:$Port/healthz" -TimeoutSec 3 -UseBasicParsing
            $ready = $true; break
        } catch { }
    }
    Assert $ready "server is up (healthz)" "no response after 24s; see $dataPath\server.err.log"
    if (-not $ready) { throw "server never became ready" }

    $script:session = New-Object Microsoft.PowerShell.Commands.WebRequestSession

    # --- onboard ------------------------------------------------------------
    Write-Step "onboard first admin"
    $status = Invoke-Json GET "/api/status"
    Assert ($status.ok -eq $true) "GET /api/status ok"
    Assert ($status.configured -eq $false) "fresh instance reports unconfigured"

    $onboard = Invoke-Json POST "/api/onboard" @{
        username = "smoke"; email = "smoke@example.com"
        password = "SmokeTest123"; displayName = "Smoke"
    }
    Assert ($onboard.ok -eq $true) "POST /api/onboard ok" ($onboard.error)
    Assert ($onboard.user.role -eq "admin") "first account is admin"
    $script:adminId = $onboard.user.id

    # --- engine -------------------------------------------------------------
    Write-Step "engine status + self-test"
    $engine = Invoke-Json GET "/api/interview/engine"
    Assert ($engine.ok -eq $true) "GET /api/interview/engine ok"
    Assert ($null -ne $engine.engine.model) "engine reports a model" ($engine.engine | ConvertTo-Json -Compress)
    Assert ($engine.engine.apiKeyMasked.Length -lt 20) "key is masked, not echoed back" $engine.engine.apiKeyMasked
    Assert ($engine.engine.apiKeySet -eq $true) "the engine reports that a key is set"
    Assert ($null -ne $engine.engine.temperature) "the engine reports the effective temperature"
    Write-Host "        provider=$($engine.engine.provider) model=$($engine.engine.model) source=$($engine.engine.source)"

    # The probe moved behind the admin role: model config is admin-only now,
    # so testing a model is too.
    $probe = Invoke-Json POST "/api/admin/settings/test" @{}
    Assert ($probe.ok -eq $true) "POST /api/admin/settings/test answered"
    if ($probe.result.ok) {
        Write-Host "        live model replied in $($probe.result.latencyMs)ms" -ForegroundColor Green
    } else {
        Write-Host "        model unavailable, offline fallback expected: $($probe.result.error)" -ForegroundColor Yellow
    }

    # --- presets ------------------------------------------------------------
    Write-Step "presets + labels"
    $presets = Invoke-Json GET "/api/interview/presets"
    Assert ($presets.ok -eq $true) "GET /api/interview/presets ok"
    Assert ($presets.presets.Count -ge 6) "at least 6 presets" "got $($presets.presets.Count)"
    Assert ($presets.labels.level.Count -ge 4) "level labels present"
    Assert ($presets.labels.recommendation.Count -ge 4) "recommendation labels present"

    # --- create -------------------------------------------------------------
    Write-Step "create interview"
    $created = Invoke-Json POST "/api/interviews" @{
        role = "高级后端工程师"; level = "senior"; interviewType = "tech"
        language = "zh"; difficulty = "normal"; questionCount = 3
        jdText = "负责核心交易系统的架构设计与性能优化，要求精通 Go 与 MySQL，有高并发经验。"
        resumeText = "5 年后端经验，主导过日订单 300 万的交易系统重构，熟悉 Go/MySQL/Redis/Kafka。"
    }
    Assert ($created.ok -eq $true) "POST /api/interviews ok" ($created.error)
    $ivId = $created.interview.id
    Assert ($ivId -like "iv_*") "interview id looks right" $ivId
    Assert ($created.interview.status -eq "draft") "new interview is a draft"
    Assert ($created.interview.questionCount -eq 3) "questionCount honoured"

    $list = Invoke-Json GET "/api/interviews"
    Assert ($list.interviews.Count -eq 1) "GET /api/interviews returns the row"
    Assert ($null -eq $list.interviews[0].plan) "list DTO omits the plan"
    Assert ($list.interviews[0].resumeText -eq "") "list DTO omits the resume"

    # --- advance: start -----------------------------------------------------
    Write-Step "advance: start (plan + first question)"
    $raw = Invoke-Api POST "/api/interviews/$ivId/advance" @{ action = "start" }
    $frames = Parse-Frames $raw
    $types = ($frames | ForEach-Object { $_.type }) -join ","
    if ($ShowFrames) { Write-Host "        frames: $types" }
    Assert ($types -match "plan") "start emits a plan frame" $types
    Assert ($types -match "question") "start emits a question frame" $types
    Assert ($types -match "done") "start ends with done" $types
    Assert ($types -notmatch "(^|,)error(,|$)") "start has no error frame" $types

    $plan = ($frames | Where-Object { $_.type -eq "plan" } | Select-Object -First 1).data
    Assert ($plan.questions.Count -eq 3) "plan has the requested 3 questions" "got $($plan.questions.Count)"
    Assert ($plan.dimensions.Count -ge 3) "plan carries a rubric" "got $($plan.dimensions.Count)"

    $q1 = ($frames | Where-Object { $_.type -eq "question" } | Select-Object -First 1).data.turn
    Assert ($null -ne $q1.question) "first question has text"
    Assert ($q1.seq -eq 1) "first question is seq 1"
    Write-Host "        Q1: $($q1.question)"

    # turn_count is a denormalised column: it must agree with the turns the
    # final frame reports, or list pages and analytics undercount.
    $doneIv = ($frames | Where-Object { $_.type -eq "done" } | Select-Object -First 1).data.interview
    $doneTurns = ($frames | Where-Object { $_.type -eq "done" } | Select-Object -First 1).data.turns
    Assert ($doneIv.turnCount -eq $doneTurns.Count) "turnCount matches the turn list after start" "turnCount=$($doneIv.turnCount) turns=$($doneTurns.Count)"
    Assert ($doneIv.turnCount -eq 1) "turnCount is 1 after the first question" "got $($doneIv.turnCount)"

    # --- advance: answer ----------------------------------------------------
    Write-Step "advance: answer (grade + next question)"
    $raw = Invoke-Api POST "/api/interviews/$ivId/advance" @{
        action = "answer"; elapsedSec = 95
        answer = "MySQL 索引默认是 B+树，叶子节点存数据、非叶子只存键，所以范围查询和排序效率高。" +
                 "回表是二级索引拿到主键后再查聚簇索引，代价是两次 B+树 查找，用覆盖索引可以避免。" +
                 "复合索引要遵守最左前缀原则。取舍上，索引会带来写入放大，所以我在项目里对" +
                 "写多读少的日志表只保留主键。我们交易系统把 P99 从 480 毫秒降到了 90 毫秒。"
    }
    $frames = Parse-Frames $raw
    $types = ($frames | ForEach-Object { $_.type }) -join ","
    Assert ($types -match "grade") "answer emits a grade frame" $types
    Assert ($types -match "done") "answer ends with done" $types
    Assert ($types -notmatch "(^|,)error(,|$)") "answer has no error frame" $types

    $grade = ($frames | Where-Object { $_.type -eq "grade" } | Select-Object -First 1).data.turn
    Assert ($grade.score -gt 0) "the answer was scored above zero" "score=$($grade.score)"
    Assert ($null -ne $grade.grade.dimensions) "grade carries per-dimension scores"
    Assert ($grade.answerSeconds -eq 95) "elapsed time was recorded"
    Write-Host "        scored $($grade.score) in $($grade.answerSeconds)s, next: $(($frames | Where-Object { $_.type -eq 'question' } | Select-Object -First 1).data.turn.question)"

    # --- advance: skip ------------------------------------------------------
    Write-Step "advance: skip"
    $raw = Invoke-Api POST "/api/interviews/$ivId/advance" @{ action = "skip" }
    $frames = Parse-Frames $raw
    $skipped = ($frames | Where-Object { $_.type -eq "grade" } | Select-Object -First 1).data.turn
    Assert ($skipped.score -eq 0) "a skipped question scores 0" "score=$($skipped.score)"
    Assert ($null -ne $skipped.answeredAt) "a skipped question counts as answered"
    $skipDone = ($frames | Where-Object { $_.type -eq "done" } | Select-Object -First 1).data
    Assert ($skipDone.interview.turnCount -eq $skipDone.turns.Count) "turnCount tracks the turn list after a skip" "turnCount=$($skipDone.interview.turnCount) turns=$($skipDone.turns.Count)"
    # "Open" means answeredAt is null, NOT answer == "": a skipped turn keeps
    # an empty answer but is answered, which is exactly why the server
    # defines the pending turn by the timestamp.
    $open = ($skipDone.turns | Where-Object { $null -eq $_.answeredAt }).Count
    Assert ($open -le 1) "at most one turn is still open" "got $open"
    $skippedTurn = ($skipDone.turns | Where-Object { $_.seq -eq 2 })[0]
    Assert ($null -ne $skippedTurn.answeredAt) "the skipped turn is answered, not open"
    Assert ($skippedTurn.answer -eq "") "the skipped turn carries no answer text"

    # --- advance: finish ----------------------------------------------------
    Write-Step "advance: finish (report)"
    $raw = Invoke-Api POST "/api/interviews/$ivId/advance" @{ action = "finish" }
    $frames = Parse-Frames $raw
    $types = ($frames | ForEach-Object { $_.type }) -join ","
    Assert ($types -match "report") "finish emits a report frame" $types
    Assert ($types -match "done") "finish ends with done" $types

    $report = ($frames | Where-Object { $_.type -eq "report" } | Select-Object -First 1).data
    Assert ($report.overallScore -gt 0) "report has an overall score" "$($report.overallScore)"
    Assert ($report.recommendation -match "^(strong_hire|hire|maybe|no_hire)$") "report has a valid recommendation" $report.recommendation
    Assert ($report.dimensions.Count -ge 3) "report scores every dimension" "got $($report.dimensions.Count)"
    Assert ($report.resumeSuggestions.Count -ge 1) "report gives resume advice"
    Assert ($report.interviewStrategies.Count -ge 1) "report gives interview strategies"
    Assert ($report.strengths.Count -ge 1) "report lists strengths" "got $($report.strengths.Count)"
    Assert ($report.weaknesses -is [array]) "weaknesses rendered as an array, not null"
    Write-Host "        overall=$($report.overallScore) hr=$($report.hrSatisfaction) rec=$($report.recommendation)"

    # --- read back ----------------------------------------------------------
    Write-Step "read back interview + turns"
    $detail = Invoke-Json GET "/api/interviews/$ivId"
    Assert ($detail.ok -eq $true) "GET /api/interviews/{id} ok"
    Assert ($detail.interview.status -eq "completed") "status is completed"
    Assert ($null -ne $detail.interview.startedAt) "startedAt persisted"
    Assert ($null -ne $detail.interview.completedAt) "completedAt persisted"
    Assert ($detail.interview.overallScore -gt 0) "overallScore persisted on the row"
    Assert ($detail.interview.recommendation -ne "") "recommendation persisted on the row"
    Assert ($detail.turns.Count -ge 3) "turns persisted" "got $($detail.turns.Count)"
    Assert ($null -ne $detail.interview.plan) "detail DTO includes the plan"
    Assert ($detail.interview.resumeText -ne "") "detail DTO includes the resume"
    $answered = ($detail.turns | Where-Object { $null -ne $_.answeredAt }).Count
    Assert ($answered -ge 2) "at least two turns are answered" "got $answered"

    # --- export -------------------------------------------------------------
    Write-Step "export markdown"
    $res = Invoke-WebRequest "http://127.0.0.1:$Port/api/interviews/$ivId/export" -WebSession $session -UseBasicParsing
    # .Content is a String for textual content types and Byte[] otherwise.
    $md = if ($res.Content -is [byte[]]) { [System.Text.Encoding]::UTF8.GetString($res.Content) } else { [string]$res.Content }
    Assert ($res.StatusCode -eq 200) "export returns 200"
    Assert ($res.Headers["Content-Type"] -match "text/markdown") "export content type" $res.Headers["Content-Type"]
    Assert ($md -match "^# ") "export starts with a heading"
    Assert ($md -match "综合评分") "export contains the score section"
    Assert ($md -match "逐题记录") "export contains the per-question log"
    Assert ($md -match "简历改进建议") "export contains resume advice"
    Write-Host "        exported $($md.Length) chars of Markdown"

    # --- stats --------------------------------------------------------------
    Write-Step "stats"
    $stats = (Invoke-Json GET "/api/interview/stats").stats
    Assert ($stats.total -eq 1) "stats counts the interview" "total=$($stats.total)"
    Assert ($stats.completed -eq 1) "stats counts a completion"
    Assert ($stats.avgScore -gt 0) "stats averages the score"
    Assert ($stats.scoreByDay.Count -eq 14) "stats has the 14-day window" "got $($stats.scoreByDay.Count)"
    Assert ($stats.dimensionAvg.Count -ge 1) "stats aggregates dimensions" "got $($stats.dimensionAvg.Count)"
    Assert ($stats.recommendationBreakdown.Count -eq 4) "recommendation breakdown is a fixed legend"
    Assert ($stats.roleBreakdown.Count -ge 1) "role breakdown present"

    # --- live events --------------------------------------------------------
    Write-Step "realtime events"
    # Invoke-WebRequest hangs forever on a stream that never ends, so this
    # check drives a raw socket with an explicit read timeout: we assert the
    # SSE handshake and the hello frame arrive, then close.
    $cookie = ($script:session.Cookies.GetCookies("http://127.0.0.1:$Port") |
        ForEach-Object { "$($_.Name)=$($_.Value)" }) -join "; "
    $client = New-Object System.Net.Sockets.TcpClient
    $client.ReceiveTimeout = 4000
    $client.Connect("127.0.0.1", $Port)
    $stream = $client.GetStream()
    $stream.ReadTimeout = 4000
    $req = "GET /api/events HTTP/1.1`r`nHost: 127.0.0.1:$Port`r`nCookie: $cookie`r`n" +
           "Accept: text/event-stream`r`nConnection: close`r`n`r`n"
    $bytes = [System.Text.Encoding]::ASCII.GetBytes($req)
    $stream.Write($bytes, 0, $bytes.Length)
    $stream.Flush()
    $buf = New-Object byte[] 8192
    $read = 0
    try { $read = $stream.Read($buf, 0, $buf.Length) } catch { }
    $text = if ($read -gt 0) { [System.Text.Encoding]::UTF8.GetString($buf, 0, $read) } else { "" }
    $client.Close()

    Assert ($text -match "200") "SSE endpoint returns 200" ($text -split "`r`n" | Select-Object -First 1)
    Assert ($text -match "text/event-stream") "SSE content type is event-stream"
    Assert ($text -match '"type":"hello"') "SSE sends the hello frame" $text

    # --- guardrails ---------------------------------------------------------
    Write-Step "guardrails"
    $bad = Invoke-Json POST "/api/interviews" @{ level = "senior" }
    Assert ($bad.ok -eq $false) "creating without a role is rejected" ($bad | ConvertTo-Json -Compress)

    $update = Invoke-Json PUT "/api/interviews/$ivId" @{ role = "改不了" }
    Assert ($update.ok -eq $false) "updating a started interview is rejected" ($update | ConvertTo-Json -Compress)

    $unknown = Invoke-Json POST "/api/interviews/$ivId/advance" @{ action = "teleport" }
    Assert ($unknown.ok -eq $false) "an unknown action is rejected" ($unknown | ConvertTo-Json -Compress)

    $missing = Invoke-Json GET "/api/interviews/iv_does_not_exist"
    Assert ($missing.ok -eq $false) "an unknown id 404s" ($missing | ConvertTo-Json -Compress)

    # --- turnId binding -----------------------------------------------------
    # A late retry must be refused rather than scored against whatever
    # question happens to be open now.
    Write-Step "turnId binds a submit to its question"
    $bindIv = (Invoke-Json POST "/api/interviews" @{
        role = "后端工程师"; level = "mid"; interviewType = "tech"; questionCount = 3
    }).interview.id
    $frames = Parse-Frames (Invoke-Api POST "/api/interviews/$bindIv/advance" @{ action = "start" })
    $openTurn = ($frames | Where-Object { $_.type -eq "question" } | Select-Object -First 1).data.turn
    $mismatch = Invoke-Api POST "/api/interviews/$bindIv/advance" @{
        action = "answer"; answer = "迟到的作答"; turnId = "tr_stale_does_not_exist"
    }
    Assert ($mismatch -match "翻页") "a stale turnId is refused" ($mismatch.Substring(0, [Math]::Min(160, $mismatch.Length)))
    $mismatchFrames = Parse-Frames $mismatch
    Assert ((($mismatchFrames | Where-Object { $_.type -eq "grade" }).Count) -eq 0) "the stale submit was not graded"
    # The matching turnId still works.
    $good = Invoke-Api POST "/api/interviews/$bindIv/advance" @{
        action = "answer"; answer = "这是一个足够长的正常回答，用来验证绑定正确时会正常评分。"; turnId = $openTurn.id
    }
    $goodFrames = Parse-Frames $good
    Assert ((($goodFrames | Where-Object { $_.type -eq "grade" }).Count) -eq 1) "a matching turnId is graded"
    Assert ($good -notmatch "翻页") "the matching turnId is not rejected"

    # --- abort guards -------------------------------------------------------
    Write-Step "abort guards"
    $draftIv = (Invoke-Json POST "/api/interviews" @{
        role = "后端工程师"; level = "mid"; interviewType = "tech"; questionCount = 3
    }).interview.id
    $draftAbort = Invoke-Json POST "/api/interviews/$draftIv/advance" @{ action = "abort" }
    Assert ($draftAbort.ok -eq $false) "aborting a draft is refused" ($draftAbort | ConvertTo-Json -Compress)
    $null = Invoke-Json DELETE "/api/interviews/$draftIv" $null
    $null = Invoke-Json DELETE "/api/interviews/$bindIv" $null

    # --- admin cascades -----------------------------------------------------
    # Regression guard: the store used to cascade to the deleted template
    # `items` table, which made EVERY user delete fail with a 500.
    Write-Step "admin user delete cascades"
    $tmpUser = Invoke-Json POST "/api/users" @{
        username = "smoketmp"; email = "smoketmp@example.com"
        password = "SmokeTest123"; displayName = "Tmp"
    }
    Assert ($tmpUser.ok -eq $true) "a second account can be created" ($tmpUser | ConvertTo-Json -Compress)
    $tmpId = $tmpUser.user.id
    $delUser = Invoke-Json DELETE "/api/users/$tmpId" $null
    Assert ($delUser.ok -eq $true) "DELETE /api/users/{id} succeeds (no stale items table)" ($delUser | ConvertTo-Json -Compress)
    $usersAfter = Invoke-Json GET "/api/users"
    Assert (($usersAfter.users | Where-Object { $_.id -eq $tmpId }).Count -eq 0) "the deleted account is gone"

    # --- trivially short answer --------------------------------------------
    Write-Step "short answer is graded without calling the model"
    $shortIv = (Invoke-Json POST "/api/interviews" @{
        role = "后端工程师"; level = "mid"; interviewType = "tech"; questionCount = 3
    }).interview.id
    $null = Invoke-Api POST "/api/interviews/$shortIv/advance" @{ action = "start" }
    $raw = Invoke-Api POST "/api/interviews/$shortIv/advance" @{ action = "answer"; answer = "a" }
    $frames = Parse-Frames $raw
    $shortGrade = ($frames | Where-Object { $_.type -eq "grade" } | Select-Object -First 1).data.turn
    Assert ($shortGrade.score -eq 0) "a 1-character answer scores 0" "score=$($shortGrade.score)"
    Assert ($raw -match "未调用模型") "the fallback reason reaches the candidate" ($raw.Substring(0, [Math]::Min(200, $raw.Length)))
    Assert ($shortGrade.answerSeconds -eq 0) "answerSeconds is 0 when the client sends no elapsedSec"
    Assert ($raw -match "过于简短") "the short answer gets the honest 'too short' text"

    # A 2-3 character answer that matches a technology regex ("Go", "SQL")
    # must land in the SAME branch, not get a generic critique plus a
    # fraction of a point while the note claims it was not assessed.
    $null = Invoke-Api POST "/api/interviews/$shortIv/advance" @{ action = "answer"; answer = "Go" }
    $detail = Invoke-Json GET "/api/interviews/$shortIv"
    $goTurn = ($detail.turns | Where-Object { $_.answer -eq "Go" })[0]
    Assert ($goTurn.score -eq 0) "'Go' scores 0 like any trivial answer" "score=$($goTurn.score)"
    Assert ($goTurn.feedback -match "过于简短") "'Go' gets the same honest text" ($goTurn.feedback.Substring(0, [Math]::Min(80, $goTurn.feedback.Length)))

    # --- abort --------------------------------------------------------------
    Write-Step "abort marks the interview as given up"
    $abortRaw = Invoke-Api POST "/api/interviews/$shortIv/advance" @{ action = "abort" }
    $abortDone = (Parse-Frames $abortRaw | Where-Object { $_.type -eq "done" } | Select-Object -First 1).data
    Assert ($null -ne $abortDone) "abort ends with done"
    Assert ($abortDone.interview.status -eq "aborted") "status is aborted" $abortDone.interview.status
    Assert ($abortDone.turns.Count -ge 1) "abort keeps the recorded turns" "got $($abortDone.turns.Count)"
    $statsAbort = (Invoke-Json GET "/api/interview/stats").stats
    Assert ($statsAbort.aborted -eq 1) "stats counts the give-up" "aborted=$($statsAbort.aborted)"
    # `finish` is a streaming action, so it must be read as frames, not JSON.
    $abortFinishRaw = Invoke-Api POST "/api/interviews/$shortIv/advance" @{ action = "finish" }
    $abortFinishFrames = Parse-Frames $abortFinishRaw
    $finishDone = ($abortFinishFrames | Where-Object { $_.type -eq "done" } | Select-Object -First 1).data
    Assert ($null -ne $finishDone) "an aborted interview can still produce a report" $abortFinishRaw
    Assert ($finishDone.interview.status -eq "completed") "finishing an aborted interview moves it to completed" $finishDone.interview.status
    Assert ((($abortFinishFrames | Where-Object { $_.type -eq "report" }).Count) -eq 1) "finish emitted a report frame"

    # --- resume from aborted ------------------------------------------------
    Write-Step "restarting an aborted interview resumes it"
    $resumeIv = (Invoke-Json POST "/api/interviews" @{
        role = "后端工程师"; level = "mid"; interviewType = "tech"; questionCount = 3
    }).interview.id
    $null = Invoke-Api POST "/api/interviews/$resumeIv/advance" @{ action = "start" }
    $null = Invoke-Api POST "/api/interviews/$resumeIv/advance" @{ action = "abort" }
    $resumeFrames = Parse-Frames (Invoke-Api POST "/api/interviews/$resumeIv/advance" @{ action = "start" })
    $resumeDone = ($resumeFrames | Where-Object { $_.type -eq "done" } | Select-Object -First 1).data
    Assert ($resumeDone.interview.status -eq "in_progress") "start on an aborted interview puts it back in progress" $resumeDone.interview.status
    $resumeOpen = ($resumeFrames | Where-Object { $_.type -eq "question" } | Select-Object -First 1).data.turn
    Assert ($null -ne $resumeOpen) "resuming re-emits the open question"
    $resumeAnswer = Invoke-Api POST "/api/interviews/$resumeIv/advance" @{
        action = "answer"; answer = "恢复后继续作答，验证这条路径真的可以继续。"; turnId = $resumeOpen.id
    }
    Assert ((Parse-Frames $resumeAnswer | Where-Object { $_.type -eq "grade" }).Count -eq 1) "the resumed session is answerable again"
    $null = Invoke-Json DELETE "/api/interviews/$resumeIv" $null

    # --- delete -------------------------------------------------------------
    Write-Step "delete cascades"
    $del = Invoke-Json DELETE "/api/interviews/$ivId" $null
    Assert ($del.ok -eq $true) "DELETE /api/interviews/{id} ok"
    $null = Invoke-Json DELETE "/api/interviews/$shortIv" $null
    $after = Invoke-Json GET "/api/interviews"
    Assert ($after.interviews.Count -eq 0) "interviews are gone"
    $statsAfter = (Invoke-Json GET "/api/interview/stats").stats
    Assert ($statsAfter.total -eq 0) "stats no longer counts them"

    # ======================================================================
    # RBAC: a regular user sees only their own rows, an admin can see
    # everyone's. Every widening is admin-only, and a non-admin asking for it
    # gets an explicit 403 rather than a silently filtered list.
    # ======================================================================
    Write-Step "RBAC: create a regular user and log in as them"
    $bob = Invoke-Json POST "/api/users" @{
        username = "bob"; email = "bob@example.com"
        password = "BobTest1234"; displayName = "Bob"
    }
    Assert ($bob.ok -eq $true) "admin can create a second account" ($bob | ConvertTo-Json -Compress)
    Assert ($bob.user.role -eq "user") "the new account is a regular user, not an admin"
    $bobId = $bob.user.id

    $bobSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    # The login body field is `login` (username or email), not `username`.
    $login = Invoke-Json POST "/api/login" @{ login = "bob"; password = "BobTest1234" } -Session $bobSession
    Assert ($login.ok -eq $true) "bob can log in" ($login | ConvertTo-Json -Compress)
    $bobMe = Invoke-Json GET "/api/me" -Session $bobSession
    Assert ($bobMe.user.role -eq "user") "bob's session is a user session"

    # An admin-owned interview for bob to fail to reach.
    $adminIv = (Invoke-Json POST "/api/interviews" @{
        role = "后端工程师"; level = "mid"; interviewType = "tech"; questionCount = 3
    }).interview.id

    Write-Step "RBAC: a regular user sees only their own rows"
    $bobList = Invoke-Json GET "/api/interviews" -Session $bobSession
    Assert ($bobList.ok -eq $true) "bob can list interviews"
    Assert ($bobList.interviews.Count -eq 0) "bob sees none of the admin's interviews" "got $($bobList.interviews.Count)"

    $bobGet = Invoke-Json GET "/api/interviews/$adminIv" -Session $bobSession
    Assert ($bobGet.ok -eq $false) "bob cannot read the admin's interview"
    Assert ($bobGet.error -match "not found") "it is a 404, not a 403, so ids cannot be probed" $bobGet.error
    $bobExport = Invoke-Json GET "/api/interviews/$adminIv/export" -Session $bobSession
    Assert ($bobExport.ok -eq $false) "bob cannot export the admin's interview"

    Write-Step "RBAC: scope widening is refused for non-admins"
    $scoped = @(
        @{ n = "?all=true on the list";      m = "GET";    p = "/api/interviews?all=true" },
        @{ n = "?userId= on the list";       m = "GET";    p = "/api/interviews?userId=$script:adminId" },
        @{ n = "?all=true on the stats";     m = "GET";    p = "/api/interview/stats?all=true" },
        @{ n = "?userId= on the stats";      m = "GET";    p = "/api/interview/stats?userId=$script:adminId" },
        @{ n = "admin settings (read)";      m = "GET";    p = "/api/admin/settings" },
        @{ n = "admin settings (write)";     m = "PUT";    p = "/api/admin/settings" },
        @{ n = "admin settings (test)";      m = "POST";   p = "/api/admin/settings/test" },
        @{ n = "admin settings (clear)";     m = "DELETE"; p = "/api/admin/settings" },
        @{ n = "create a preset";            m = "POST";   p = "/api/admin/presets" },
        @{ n = "update a preset";            m = "PUT";    p = "/api/admin/presets/ps_x" },
        @{ n = "delete a preset";            m = "DELETE"; p = "/api/admin/presets/ps_x" },
        @{ n = "another user's overview";    m = "GET";    p = "/api/admin/users/$script:adminId/overview" }
    )
    foreach ($case in $scoped) {
        $body = if ($case.m -in @("PUT", "POST")) { @{} } else { $null }
        $res = Invoke-Json $case.m $case.p $body -Session $bobSession
        Assert ($res.ok -eq $false) "bob is refused: $($case.n)" ($res | ConvertTo-Json -Compress)
        # EXACT equality, not -match: the contract publishes the bare literal
        # "forbidden" (docs/API.md §6) so a client can branch on one value. A
        # substring check hid a drift where the body carried a Chinese
        # explanation appended to it.
        Assert ($res.error -ceq "forbidden") "…and it is the exact literal 'forbidden'" "got: $($res.error)"
    }

    Write-Step "RBAC: a regular user can still READ shared presets and the engine"
    $bobPresets = Invoke-Json GET "/api/interview/presets" -Session $bobSession
    Assert ($bobPresets.ok -eq $true) "bob can read the preset list"
    Assert ($bobPresets.presets.Count -ge 6) "the built-ins are visible to bob"
    $bobEngine = Invoke-Json GET "/api/interview/engine" -Session $bobSession
    Assert ($bobEngine.ok -eq $true) "bob can see which model assesses him"
    Assert ($null -ne $bobEngine.engine.model) "…including the model name"
    # The deployment's endpoint and credential are the operator's business.
    Assert ($bobEngine.engine.baseUrl -eq "") "bob does NOT see the model endpoint"
    Assert ($bobEngine.engine.apiKeyMasked -eq "") "bob does NOT see any part of the key"
    $adminEngineView = Invoke-Json GET "/api/interview/engine"
    Assert ($adminEngineView.engine.baseUrl -ne "") "an admin DOES see the endpoint"
    Assert ($adminEngineView.engine.apiKeySet -eq $true) "an admin DOES see that a key is set"

    # ======================================================================
    # The escalation that a session-only review missed: an admin-tier API KEY
    # used to keep full admin after its owner was demoted, because IsAdmin()
    # consulted the key's tier and never the owner's current role. It could
    # then re-promote itself.
    # ======================================================================
    Write-Step "RBAC: demoting an admin revokes their API KEY too"
    $tempAdmin = Invoke-Json POST "/api/users" @{
        username = "tempadmin"; email = "tempadmin@example.com"
        password = "TempAdmin1234"; displayName = "Temp Admin"; role = "admin"
    }
    Assert ($tempAdmin.user.role -eq "admin") "created a second admin"
    $tempAdminId = $tempAdmin.user.id

    # That admin issues an ADMIN-tier key for itself.
    $tempSession = New-Object Microsoft.PowerShell.Commands.WebRequestSession
    $null = Invoke-Json POST "/api/login" @{ login = "tempadmin"; password = "TempAdmin1234" } -Session $tempSession
    $issued = Invoke-Json POST "/api/apikeys" @{
        name = "temp-admin-key"; type = "admin"
    } -Session $tempSession
    Assert ($issued.ok -eq $true) "the second admin can issue an admin-tier key" ($issued | ConvertTo-Json -Compress)
    $adminKey = $issued.apiKey.key
    Assert ($adminKey.Length -gt 20) "the plaintext key is returned exactly once"

    # It works while they are an admin.
    $asAdmin = Invoke-Json GET "/api/admin/settings" -Session $tempSession
    Assert ($asAdmin.ok -eq $true) "the second admin can read settings with their session"
    $keyAsAdmin = Invoke-Api GET "/api/interviews?all=true" -Bearer $adminKey
    Assert (($keyAsAdmin | ConvertFrom-Json).ok -eq $true) "…and with the admin-tier key"

    # Now DEMOTE them using the original admin.
    $demoted = Invoke-Json PUT "/api/users/$tempAdminId" @{ role = "user" }
    Assert ($demoted.ok -eq $true) "the second admin is demoted to user" ($demoted | ConvertTo-Json -Compress)
    Assert ($demoted.user.role -eq "user") "the role change took effect"

    # The session must lose admin. (This always worked.)
    $afterDemoteSession = Invoke-Json GET "/api/admin/settings" -Session $tempSession
    Assert ($afterDemoteSession.ok -eq $false) "the demoted admin's SESSION loses admin"

    # The KEY must lose it too — this is the regression.
    $keySettings = Invoke-Api GET "/api/admin/settings" -Bearer $adminKey
    $keySettingsJSON = $keySettings | ConvertFrom-Json
    Assert ($keySettingsJSON.ok -eq $false) "the demoted admin's API KEY loses admin" "got ok=$($keySettingsJSON.ok)"
    Assert ($keySettingsJSON.error -ceq "forbidden") "…and is refused with the literal forbidden" "got: $($keySettingsJSON.error)"
    $keyAll = (Invoke-Api GET "/api/interviews?all=true" -Bearer $adminKey) | ConvertFrom-Json
    Assert ($keyAll.ok -eq $false) "the demoted key cannot widen scope to every user's records"
    $keyOverview = Invoke-Api GET "/api/admin/users/$bobId/overview" -Bearer $adminKey
    Assert (($keyOverview | ConvertFrom-Json).ok -eq $false) "the demoted key cannot read a user overview"
    $keyWrite = Invoke-Api PUT "/api/admin/settings" @{ model = "pwned-by-demoted-key" } -Bearer $adminKey
    Assert (($keyWrite | ConvertFrom-Json).ok -eq $false) "the demoted key cannot rewrite the shared model config"
    $selfPromote = Invoke-Api PUT "/api/users/$tempAdminId" @{ role = "admin" } -Bearer $adminKey
    Assert (($selfPromote | ConvertFrom-Json).ok -eq $false) "the demoted key cannot re-promote itself"
    # There is no GET /api/users/{id} (only list + mutations), so read the
    # role back through the admin overview endpoint.
    $stillUser = Invoke-Json GET "/api/admin/users/$tempAdminId/overview"
    Assert ($stillUser.overview.user.role -eq "user") "…and the role is genuinely still user" "got: $($stillUser.overview.user.role)"
    $engineAfter = Invoke-Json GET "/api/interview/engine"
    Assert ($engineAfter.engine.model -ne "pwned-by-demoted-key") "the shared model config was not rewritten"

    # A live key with a NON-admin tier stays narrow even for a real admin.
    $narrowIssued = Invoke-Json POST "/api/apikeys" @{ name = "narrow"; type = "user" }
    Assert ($narrowIssued.ok -eq $true) "an admin can issue a user-tier key"
    $narrowKey = $narrowIssued.apiKey.key
    $narrowAdmin = (Invoke-Api GET "/api/admin/settings" -Bearer $narrowKey) | ConvertFrom-Json
    Assert ($narrowAdmin.ok -eq $false) "a user-tier key held by an admin stays narrow"
    $narrowOwn = (Invoke-Api GET "/api/me" -Bearer $narrowKey) | ConvertFrom-Json
    Assert ($narrowOwn.ok -eq $true) "…but still works for non-admin endpoints"

    $null = Invoke-Json DELETE "/api/users/$tempAdminId" $null

    Write-Step "RBAC: an admin can see every user's records"
    $adminAll = Invoke-Json GET "/api/interviews?all=true"
    Assert ($adminAll.ok -eq $true) "admin ?all=true is allowed"
    Assert ($adminAll.interviews.Count -ge 1) "admin sees the row it created"
    $adminForBob = Invoke-Json GET "/api/interviews?userId=$bobId"
    Assert ($adminForBob.ok -eq $true) "admin ?userId= is allowed"
    Assert ($adminForBob.interviews.Count -eq 0) "bob has no interviews yet"
    $adminStats = Invoke-Json GET "/api/interview/stats?all=true"
    Assert ($adminStats.ok -eq $true) "admin ?all=true on stats is allowed"
    $overview = Invoke-Json GET "/api/admin/users/$bobId/overview"
    Assert ($overview.ok -eq $true) "admin can read a user overview" ($overview.error)
    Assert ($overview.overview.user.username -eq "bob") "the overview is about bob"
    Assert ($overview.overview.total -eq 0) "bob's overview counts zero interviews"
    Assert ($overview.overview.recommendationBreakdown.Count -eq 4) "the overview keeps a fixed legend"
    $adminReadsBobIv = Invoke-Json GET "/api/interviews/$adminIv"
    Assert ($adminReadsBobIv.ok -eq $true) "the admin still reads its own row"

    # ======================================================================
    # Admin: runtime model configuration. The whole point is that no
    # credential is compiled in, so this is the only way to set one.
    # ======================================================================
    Write-Step "admin settings: read, write, numeric clear, reset"
    $initial = Invoke-Json GET "/api/admin/settings"
    Assert ($initial.ok -eq $true) "admin can read settings"
    Assert ($initial.settings.apiKeySet -eq $true) "a key is configured (from .env)" "apiKeySet=$($initial.settings.apiKeySet)"
    Assert ($initial.settings.configured -eq $true) "the model is configured (from .env)"
    Assert ($initial.settings.overridden.baseUrl -ne $true) "nothing is overridden in the DB yet"
    Assert ($initial.settings.envPresent.Count -ge 1) "the env vars in use are reported" ($initial.settings.envPresent -join ",")
    Assert ($null -eq $initial.settings.apiKey) "the raw key field must not exist at all"
    $maskedLen = "$($initial.settings.apiKeyMasked)".Length
    Assert ($maskedLen -lt 20) "the key is only ever masked" "masked=$($initial.settings.apiKeyMasked)"

    # Override two fields in the DB and prove they take effect.
    $put = Invoke-Json PUT "/api/admin/settings" @{
        model = "admin-chosen-model"; maxTokens = 4096; timeoutSec = 45
    }
    Assert ($put.ok -eq $true) "admin can write settings" ($put.error)
    Assert ($put.settings.model -eq "admin-chosen-model") "the stored model is returned"
    Assert ($put.settings.overridden.model -eq $true) "model is now marked as a DB override"
    Assert ($put.settings.overridden.maxTokens -eq $true) "maxTokens is now marked as a DB override"
    Assert ($put.settings.maxTokens -eq 4096) "the numeric value took effect"
    $engineNow = Invoke-Json GET "/api/interview/engine"
    Assert ($engineNow.engine.model -eq "admin-chosen-model") "the read-only engine view reflects the override"
    Assert ($engineNow.engine.source -eq "db") "source reports the database" $engineNow.engine.source

    # A numeric field must be clearable with "" — the documented uniform rule.
    $cleared = Invoke-Json PUT "/api/admin/settings" @{ maxTokens = "" }
    Assert ($cleared.ok -eq $true) "a numeric override can be cleared with an empty string" ($cleared.error)
    Assert ($cleared.settings.overridden.maxTokens -ne $true) "maxTokens is no longer a DB override"
    Assert ($cleared.settings.maxTokens -eq 8192) "it fell back to the env/default value" "got $($cleared.settings.maxTokens)"
    Assert ($cleared.settings.overridden.model -eq $true) "clearing one field left the other alone"

    $badProvider = Invoke-Json PUT "/api/admin/settings" @{ provider = "nonsense" }
    Assert ($badProvider.ok -eq $false) "an unknown provider is rejected" ($badProvider | ConvertTo-Json -Compress)
    $badRange = Invoke-Json PUT "/api/admin/settings" @{ maxTokens = 999999999 }
    Assert ($badRange.ok -eq $false) "an out-of-range maxTokens is rejected" ($badRange | ConvertTo-Json -Compress)

    $reset = Invoke-Json DELETE "/api/admin/settings" @{}
    Assert ($reset.ok -eq $true) "admin can clear all overrides" ($reset.error)
    Assert ($reset.settings.overridden.model -ne $true) "no field is overridden after the reset"
    $engineReset = Invoke-Json GET "/api/interview/engine"
    Assert ($engineReset.engine.model -ne "admin-chosen-model") "the model reverted to the env value"
    Assert ($engineReset.engine.configured -eq $true) "the env/.env model still applies"

    # ======================================================================
    # Admin: shared presets
    # ======================================================================
    Write-Step "admin presets: create, list, update, delete"
    $newPreset = Invoke-Json POST "/api/admin/presets" @{
        role = "资深数据工程师"; level = "senior"; interviewType = "tech"
        difficulty = "hard"; questionCount = 8
        focusAreas = @("数仓分层", "数据质量", "调度")
        jdSample = "负责数据平台建设与治理。"; description = "偏工程"
    }
    Assert ($newPreset.ok -eq $true) "admin can create a preset" ($newPreset.error)
    Assert ($newPreset.preset.builtin -eq $false) "an admin preset is not marked builtin"
    Assert ($newPreset.preset.focusAreas.Count -eq 3) "focus areas round-trip"
    $presetId = $newPreset.preset.id

    $list = Invoke-Json GET "/api/interview/presets"
    $builtins = ($list.presets | Where-Object { $_.builtin -eq $true }).Count
    $custom = ($list.presets | Where-Object { $_.builtin -eq $false }).Count
    Assert ($builtins -ge 6) "the built-in presets are all present" "got $builtins"
    Assert ($custom -eq 1) "the admin preset appears once" "got $custom"
    # Everyone (including bob) sees the admin's preset: it is shared.
    $bobSees = Invoke-Json GET "/api/interview/presets" -Session $bobSession
    Assert (($bobSees.presets | Where-Object { $_.id -eq $presetId }).Count -eq 1) "bob sees the shared preset"

    $upd = Invoke-Json PUT "/api/admin/presets/$presetId" @{ questionCount = 5; description = "改过了" }
    Assert ($upd.ok -eq $true) "admin can update a preset" ($upd.error)
    Assert ($upd.preset.questionCount -eq 5) "the update persisted"
    Assert ($upd.preset.role -eq "资深数据工程师") "an absent field was left alone"

    $builtinId = ($list.presets | Where-Object { $_.builtin -eq $true } | Select-Object -First 1).id
    $editBuiltin = Invoke-Json PUT "/api/admin/presets/$builtinId" @{ role = "改内置" }
    Assert ($editBuiltin.ok -eq $false) "editing a built-in preset is refused" ($editBuiltin | ConvertTo-Json -Compress)
    $delBuiltin = Invoke-Json DELETE "/api/admin/presets/$builtinId" $null
    Assert ($delBuiltin.ok -eq $false) "deleting a built-in preset is refused" ($delBuiltin | ConvertTo-Json -Compress)
    $stillThere = Invoke-Json GET "/api/interview/presets"
    Assert (($stillThere.presets | Where-Object { $_.id -eq $builtinId }).Count -eq 1) "the built-in survived the attempt"

    $del = Invoke-Json DELETE "/api/admin/presets/$presetId" $null
    Assert ($del.ok -eq $true) "admin can delete their own preset" ($del.error)
    $afterDel = Invoke-Json GET "/api/interview/presets"
    Assert (($afterDel.presets | Where-Object { $_.builtin -eq $false }).Count -eq 0) "the admin preset is gone"

    # --- cleanup ------------------------------------------------------------
    $null = Invoke-Json DELETE "/api/interviews/$adminIv" $null
}
finally {
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        Write-Host "`n(server stopped)" -ForegroundColor DarkGray
    }
    if (-not $KeepData -and (Test-Path $dataPath)) {
        Remove-Item -Recurse -Force $dataPath -ErrorAction SilentlyContinue
    }
}

Write-Host ""
if ($script:failures -eq 0) {
    Write-Host "SMOKE TEST PASSED — $($script:checks)/$($script:checks) checks" -ForegroundColor Green
    exit 0
}
Write-Host "SMOKE TEST FAILED — $($script:failures) of $($script:checks) checks failed" -ForegroundColor Red
exit 1
