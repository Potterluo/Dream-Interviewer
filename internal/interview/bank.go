package interview

import "strings"

// bank.go: the curated offline catalogue — the questions, the role
// presets and the role→family classifier.
//
// WHY this file exists at all: the offline path is what the product
// falls back to when there is no API key, no network or no account, so
// the questions here have to stand on their own as a real interview.
// That is why every entry carries Intent (what the question probes) and
// Reference (what a strong answer covers) and why Keywords are chosen
// from the vocabulary a strong answer actually uses — the deterministic
// grader scores keyword coverage, so a lazy keyword list silently makes
// grading wrong rather than merely vague.
//
// The catalogue is a package-level literal so it is built exactly once
// at init and never reallocated per call.

// Family slugs. A family is a coarse job cluster, deliberately coarser
// than a role: "Golang 后端" and "Java 架构师" share one question pool
// because the depth questions do not care about the language.
const (
	FamilyBackend   = "backend"
	FamilyFrontend  = "frontend"
	FamilyFullstack = "fullstack"
	FamilyData      = "data"
	FamilyAlgorithm = "algorithm"
	FamilyProduct   = "product"
	FamilyOps       = "ops"
	FamilyTest      = "test"
	FamilyGeneric   = "generic"
)

// BankQuestion is one curated question.
//
// Levels lists every seniority the question suits — questions are reused
// across levels on purpose (the same index question is a junior screen
// and, with the follow-ups, a senior probe), so a family's questions
// must union to all four levels.
type BankQuestion struct {
	Family    string   // one of the Family* slugs
	Dimension string   // a Dim* constant from types.go
	Levels    []string // any of LevelJunior/LevelMid/LevelSenior/LevelExpert this suits
	Question  string   // the question text, in Chinese
	Intent    string   // one short Chinese sentence: what this question probes
	Reference string   // 2-4 Chinese sentences: what a strong answer covers
	Keywords  []string // 6-12 lowercase tokens (Chinese terms and/or English tech words) a strong answer would mention; used by the deterministic grader for keyword-coverage scoring
}

// Preset is a one-click interview setup shown in the "新建面试" wizard.
// The JSON tags are part of the wire contract with the frontend.
type Preset struct {
	ID            string   `json:"id"`
	Role          string   `json:"role"`
	Level         string   `json:"level"`
	InterviewType string   `json:"interviewType"`
	Difficulty    string   `json:"difficulty"`
	QuestionCount int      `json:"questionCount"`
	FocusAreas    []string `json:"focusAreas"`
	JDSample      string   `json:"jdSample"`
	Description   string   `json:"description"`
}

// bankCatalogue is the whole built-in question pool. Nine families, and
// every family covers all four levels and at least three dimensions.
var bankCatalogue = []BankQuestion{
	// --- backend ---------------------------------------------------------
	{
		Family:    FamilyBackend,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "MySQL 的订单表要按 user_id 查询该用户最近的 10 条订单，你会怎么建索引？为什么 B+ 树索引能高效支持这种范围查询？",
		Intent:    "考察索引设计与 B+ 树原理的基本功。",
		Reference: "先说清应建 (user_id, created_at) 的联合索引：最左前缀定位到 user_id，再在索引内按 created_at 逆序取 10 条即可，不需要排序和全表扫描。再讲 InnoDB 的 B+ 树只有叶子节点存数据、叶子之间用双向链表相连，所以范围查询只需定位起点后顺序扫描，且树高三层左右就能覆盖千万级数据。最后补充取舍：联合索引的顺序由查询与排序决定，区分度低的列放前面反而浪费，必要时用覆盖索引避免回表。",
		Keywords:  []string{"联合索引", "最左前缀", "b+树", "回表", "覆盖索引", "innodb", "行锁", "explain", "区分度", "聚簇索引"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "请说明 MySQL 的四种事务隔离级别，以及 InnoDB 在可重复读级别下是怎么处理幻读的？",
		Intent:    "考察事务隔离、MVCC 与锁机制是否成体系。",
		Reference: "先把四个级别说全（读未提交、读已提交、可重复读、串行化）并指出各自能解决脏读、不可重复读、幻读中的哪些。接着说 InnoDB 的实现：读已提交每次查询生成新的 Read View，可重复读在事务首次快照读时生成并复用，配合 undo log 得到一致性快照。然后讲快照读靠 MVCC 解决幻读，当前读（select for update、update、insert）靠 Next-Key Lock 和间隙锁阻止其他事务插入；能指出可重复读下幻读仍可能在「先快照读再当前读」的场景出现，就是深度。",
		Keywords:  []string{"读未提交", "读已提交", "可重复读", "串行化", "mvcc", "read view", "undo log", "间隙锁", "next-key lock", "快照读", "当前读"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "缓存穿透、击穿和雪崩分别是什么？针对每一种给出一个线上可落地的方案。",
		Intent:    "考察缓存问题的区分能力与工程方案成熟度。",
		Reference: "先明确三者边界：穿透是查不存在的数据、击穿是单个热点 key 过期、雪崩是大量 key 同时失效或缓存整体不可用。穿透可用布隆过滤器拦截非法 key，并对查不到的结果写入短 TTL 的空值；击穿用互斥锁重建或逻辑过期 + 异步刷新，保证只有一个请求回源；雪崩给过期时间加随机抖动、做多级缓存和集群化，并准备好熔断降级兜底。能补一句缓存和数据库的一致性策略（先更新库再删缓存、延迟双删）会更完整。",
		Keywords:  []string{"缓存穿透", "缓存击穿", "缓存雪崩", "布隆过滤器", "空值缓存", "互斥锁", "逻辑过期", "随机过期时间", "热点key", "熔断降级"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimAccuracy,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "用 Redis 实现分布式锁，SETNX 方案有哪些坑？如何保证只有锁的持有者能释放锁？你怎么看 Redlock 的争议？",
		Intent:    "考察对分布式锁边界条件的理解和批判性判断。",
		Reference: "要点是「加锁必须原子」：用 SET key value NX PX ttl 一条命令，不要先 SETNX 再 EXPIRE。释放锁必须用 Lua 脚本比对唯一 value（如 requestId + 线程标识）后删除，否则可能删掉别人重新加上的锁。业务执行超过过期时间会失去锁，需要看门狗续期或 fencing token 保证下游拒绝过期写。Redlock 的争议在于它依赖各节点时钟不大幅漂移、且没有 fencing 时无法对抗长 GC 停顿，多数团队更愿意用带租约和版本号的方案，或直接依赖 etcd/ZooKeeper 的临时顺序节点。",
		Keywords:  []string{"setnx", "set nx px", "过期时间", "lua", "唯一value", "看门狗", "续期", "redlock", "时钟漂移", "fencing token", "可重入"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "支付回调接口如何保证幂等？请给出至少两种方案，并说明各自的适用边界。",
		Intent:    "考察幂等设计的实操经验，而不只是背概念。",
		Reference: "常见方案有三类：一是唯一索引/去重表，用订单号 + 回调来源或业务流水号建唯一键，插入冲突即视为重复，简单可靠但要处理并发插入异常；二是状态机 + 乐观锁，把订单从待支付改为已支付时带 where status = '待支付'，更新影响行数为 0 直接返回成功；三是分布式锁或幂等 token，在入口按业务号串行化，代价是性能与可用性。要点是幂等键必须由业务唯一确定、失败与重复要有区分，并且重复请求要返回与首次一致的结果，否则上游会一直重试。",
		Keywords:  []string{"幂等", "唯一索引", "去重表", "状态机", "乐观锁", "版本号", "幂等token", "重试", "并发", "最终一致性"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "订单单表涨到两亿行，你会怎么分库分表？分片键怎么选，全局唯一 ID 用什么方案？",
		Intent:    "考察海量数据下的架构取舍，而不是照搬方案名。",
		Reference: "先做判断：能不能靠归档冷数据、读写分离、分区表缓解，非要分片时再分。分片键通常选 user_id 或 merchant_id，因为绝大多数查询带这个维度；风险是运营侧的跨用户查询会变成全分片扫描，需要异构索引或宽表补位。ID 可用雪花算法（注意时钟回拨）或号段模式（数据库批量取号，性能好、依赖中心服务）。还要讲清扩容：成倍扩容配合双写迁移或一致性哈希，以及分片后跨片事务、分页、聚合的代价。",
		Keywords:  []string{"分库分表", "分片键", "路由", "雪花算法", "号段模式", "时钟回拨", "扩容", "双写", "数据迁移", "一致性哈希", "跨片查询"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "Go 的 GC 是怎么工作的？三色标记法为什么需要写屏障？",
		Intent:    "考察对 Go 运行时与并发标记的正确理解。",
		Reference: "Go 用并发三色标记加混合写屏障：白色是未扫描、灰色是待扫描、黑色是已扫描完成，标记结束后白色对象被回收。并发标记时用户协程可能把黑色对象指向白色对象，或让灰色对象丢失引用，破坏「不丢对象」的不变量，因此需要写屏障记录引用变化（Go 1.8 起是混合写屏障，避免标记终止阶段的重新扫描）。还要知道 GC 触发条件（内存增长比例 GOGC、定时、手动）、STW 只发生在标记开始和结束的极短阶段，以及用 pprof、GODEBUG=gctrace 观察暂停与内存逃逸。",
		Keywords:  []string{"三色标记", "写屏障", "混合写屏障", "stw", "并发标记", "gc触发", "gogc", "内存逃逸", "pprof", "gctrace"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "消息队列出现重复消费、消息乱序和大量堆积，你分别怎么处理？",
		Intent:    "考察 MQ 三个经典故障场景的处置能力。",
		Reference: "重复消费从「至少一次」语义出发，消费端必须幂等：唯一键去重、状态机判断或 Redis setnx 记录已处理的消息 ID。乱序要先确认根源是分区策略还是并发消费，同一业务键必须路由到同一分区，或按版本号/时间戳做覆盖写而不是累加。堆积要看是消费能力不足还是消费卡死：先扩消费者并提高单批拉取量，同时用监控确认延迟在收敛；若上游写入暴涨则限流或降级，把非核心消息转入死信队列延后处理，并确认没有单条消息反复重试阻塞分区。",
		Keywords:  []string{"重复消费", "幂等", "顺序消息", "分区", "堆积", "消费者扩容", "死信队列", "ack", "重试", "限流", "kafka"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "一条慢 SQL 你是如何定位并优化的？EXPLAIN 结果里你最关注哪些字段？",
		Intent:    "考察从现象到索引的完整排查链路。",
		Reference: "先定位：开慢查询日志或看 APM 里的 SQL 耗时分布，用 pt-query-digest 排序，确认是个例还是高频。再用 EXPLAIN 看 type（all、index、range、ref 的差别）、key 与实际使用的索引、rows 估算行数、Extra 里的 filesort、temporary、using index condition，据此判断是全表扫描、索引失效还是排序代价高。优化方向包括补最左前缀匹配的联合索引、用覆盖索引消除回表、改写函数包列或隐式类型转换、把大分页换成游标、拆分大事务，最后回归压测确认 p99 真的下降。",
		Keywords:  []string{"慢查询日志", "explain", "type", "rows", "key", "extra", "filesort", "回表", "索引下推", "隐式转换", "最左前缀"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "跨服务扣库存和创建订单如何保证一致性？说说你实际选过的方案和它付出的代价。",
		Intent:    "考察分布式一致性方案的落地经验与代价意识。",
		Reference: "首选是把扣库存和创建订单放在同一事务里，避免跨服务；必须拆开时用最终一致性：本地消息表或 RocketMQ 事务消息，保证「本地事务成功则消息一定发出」，消费端幂等重试，配合定时对账补偿漏单。强一致场景可用 TCC，但要求业务提供 Try/Confirm/Cancel 三个接口并处理悬挂与空回滚，开发与运维成本高。要点是能说出所选方案在一致性、可用性、复杂度上的取舍，以及失败补偿、人工兜底和对账报表的设计。",
		Keywords:  []string{"本地消息表", "事务消息", "tcc", "saga", "补偿", "对账", "最终一致性", "幂等", "rocketmq", "空回滚", "悬挂"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimCommunication,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "HTTP 三次握手和 TLS 握手的主要步骤是什么？长连接和连接池在高并发下的价值在哪？",
		Intent:    "考察网络基础表达是否清晰、术语是否准确。",
		Reference: "三次握手：客户端发 SYN，服务端回 SYN+ACK，客户端再回 ACK，目的是双向确认收发能力并同步序列号。TLS 握手（1.2）包含 ClientHello、ServerHello 与证书、密钥交换参数、Finished 校验收尾，非对称加密只用于协商会话密钥，业务数据走对称加密；TLS 1.3 把往返压缩到一次。长连接省掉反复建连的开销，连接池则复用已建立的连接并限制并发上限，避免 TIME_WAIT 堆积和下游被拖垮，同时要配好超时、健康检查与最大空闲数。",
		Keywords:  []string{"三次握手", "syn", "ack", "tls", "证书", "对称加密", "非对称加密", "keep-alive", "连接池", "time_wait", "超时"},
	},
	{
		Family:    FamilyBackend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelExpert},
		Question:  "设计一个每秒十万次下单的秒杀系统，请说明你的限流、库存扣减与防刷方案。",
		Intent:    "考察高并发系统设计的整体收敛能力。",
		Reference: "先自顶向下拆解：静态资源与详情页走 CDN 和本地缓存，入口层用网关限流（令牌桶按用户/接口维度），无效流量在答题、验证码、黑名单阶段就被挡掉。库存预热到 Redis，用 Lua 脚本原子判断并扣减，扣减成功才生成异步下单消息，DB 侧用唯一索引和乐观锁防超卖，并做少卖兜底与对账。前端做按钮防重与排队页，压测确认瓶颈在网关还是 DB，并准备好降级、熔断和回滚预案。",
		Keywords:  []string{"限流", "令牌桶", "redis", "lua", "库存预热", "异步下单", "削峰", "超卖", "乐观锁", "幂等", "降级"},
	},

	// --- frontend --------------------------------------------------------
	{
		Family:    FamilyFrontend,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "用 CSS 让一个不定宽高的元素水平垂直居中，你能说出几种方案？Flex 和 Grid 在布局思路上有什么区别？",
		Intent:    "考察 CSS 布局基本功与方案取舍意识。",
		Reference: "常见方案：Flex 容器加 justify-content 和 align-items 居中；Grid 用 place-items: center；绝对定位加 left/top 50% 再 transform: translate(-50%, -50%)；也可用 margin: auto 配合绝对定位与四边为 0。Flex 是一维布局，主轴与交叉轴由 flex-direction 决定，适合行或列的排列与分配；Grid 是二维布局，能直接声明行列和区域模板，适合整体页面骨架。回答能说出「一维 vs 二维」和兼容性、父容器高度的前提，就算讲清了。",
		Keywords:  []string{"flex", "grid", "justify-content", "align-items", "place-items", "absolute", "transform", "margin auto", "主轴", "交叉轴", "一维布局", "二维布局"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimDepth,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "说说 JavaScript 的事件循环：宏任务和微任务分别有哪些？setTimeout 和 Promise 的先后顺序是怎么决定的？",
		Intent:    "考察异步模型是否理解到执行栈与任务队列层。",
		Reference: "JS 是单线程的，调用栈空闲后事件循环先从微任务队列取空，再取一个宏任务执行，然后进入渲染阶段（浏览器）。微任务包括 Promise.then、queueMicrotask、MutationObserver，宏任务包括 setTimeout、setInterval、I/O、UI 事件。所以同一轮里 Promise.then 一定先于 setTimeout 执行，即使延时为 0；async 函数中 await 之后的代码等价于 then 回调。能补充渲染时机、最长帧预算和 Promise 链的微任务膨胀会更完整。",
		Keywords:  []string{"事件循环", "宏任务", "微任务", "promise", "settimeout", "调用栈", "任务队列", "渲染", "async await", "queuemicrotask"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "React 列表渲染里的 key 为什么不能用数组下标？diff 过程是怎么决策复用还是重建的？",
		Intent:    "考察对 React 调和过程的理解深度。",
		Reference: "key 是节点在同层兄弟中的身份标识，React 靠它判断新旧节点是否同一实例。用下标做 key 时，在头部插入或删除会让同一 key 对应到不同数据，React 复用错误的 DOM 与组件状态，导致输入框内容错位、动画异常甚至多余请求。正确做法是用业务唯一 ID。diff 的策略是只比较同层、类型不同直接重建子树、类型相同则更新属性并递归；列表在有 key 时按 key 建立映射做最小移动。能提到 Fiber 架构下的可中断渲染与副作用提交就更深入。",
		Keywords:  []string{"react", "key", "diff", "调和", "fiber", "复用", "状态丢失", "虚拟dom", "同层比较", "最小移动"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "页面首屏加载要 5 秒，你会按什么顺序排查和优化？怎么判断优化是否真的有效？",
		Intent:    "考察性能优化的方法论，而不只是罗列手段。",
		Reference: "先用工具定位：Lighthouse 与 Performance 面板看 LCP、FCP、TBT，Network 看瀑布图，确认瓶颈是体积、请求数、TTFB 还是主线程阻塞。优化按收益排序：路由级代码分割与懒加载、依赖体积治理（tree shaking、按需引入）、静态资源上 CDN 并开 gzip/brotli、接口合并与预取、首屏关键数据 SSR 或骨架屏、图片懒加载与 WebP。最后要用同一台设备、同一网络重新跑分和真实用户监控（RUM 的 p75 LCP）对比，否则只是主观感觉变快。",
		Keywords:  []string{"lcp", "首屏", "代码分割", "懒加载", "tree shaking", "cdn", "gzip", "brotli", "骨架屏", "预取", "瀑布图"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimAccuracy,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "强缓存和协商缓存分别靠哪些响应头？为什么改了文件名就不必再操心缓存失效？",
		Intent:    "考察浏览器缓存机制与工程化实践的结合。",
		Reference: "强缓存由 Cache-Control 的 max-age、immutable 与已废弃的 Expires 控制，命中时不发请求；协商缓存由 ETag/If-None-Match 与 Last-Modified/If-Modified-Since 控制，命中时服务端返回 304，省的是响应体而不是请求。工程做法是 HTML 不缓存或短缓存、静态资源带内容哈希并设长缓存加 immutable，这样发布即新文件名，天然绕过缓存；ETag 要注意集群多节点生成一致性，否则会反复 304 失效。",
		Keywords:  []string{"cache-control", "expires", "etag", "last-modified", "304", "协商缓存", "强缓存", "内容哈希", "immutable", "max-age"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "同源策略限制的到底是什么？CORS 预检请求在什么情况下触发，带 Cookie 的跨域要怎么配置？",
		Intent:    "考察对浏览器安全模型与跨域治理的准确理解。",
		Reference: "同源指协议、域名、端口都相同，限制的核心是脚本读取跨源响应与跨源 Cookie，而不是「不能发请求」——简单请求确实发出去了，只是响应被拦截。非简单请求（方法是 PUT/DELETE，或 Content-Type 为 application/json、带自定义头）会先发 OPTIONS 预检，服务端需返回 Access-Control-Allow-Methods、Allow-Headers，并可用 Max-Age 缓存预检。带 Cookie 时必须把 Access-Control-Allow-Origin 写成具体域名而不能是星号，同时设置 Allow-Credentials 为 true，前端请求加 credentials 或 withCredentials。",
		Keywords:  []string{"同源策略", "cors", "预检", "options", "access-control-allow-origin", "credentials", "cookie", "简单请求", "代理", "自定义头"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "闭包是什么？它和内存泄漏有什么关系？var 和 let 在循环里绑定表现为什么不同？",
		Intent:    "考察作用域与变量绑定的底层理解。",
		Reference: "闭包是函数与其词法作用域的组合，内层函数访问了外层变量，外层执行结束后该变量仍被引用而无法回收。这不等于泄漏：只要闭包本身可回收就没问题，真正的泄漏是长生命周期对象（全局变量、事件监听、定时器、DOM 引用）长期持有闭包，所以要记得解绑和清理。var 是函数作用域、循环共享同一个绑定，异步回调里拿到的是最后一次的值；let 是块级作用域、每次迭代创建新绑定，因此输出 0..4。可以补充 TDZ 与 IIFE 的历史写法。",
		Keywords:  []string{"闭包", "作用域链", "内存泄漏", "var", "let", "块级作用域", "iife", "垃圾回收", "事件监听", "定时器"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "一个中台项目要接入 5 个业务团队的前端，你会怎么做微前端或模块拆分？",
		Intent:    "考察多团队协作下的架构决策与治理能力。",
		Reference: "先判断是否真需要微前端：如果只是共享组件，发布独立的 npm 包或 monorepo 更省事；只有发布节奏必须解耦、团队技术栈不同、应用边界清晰时才上。方案上可考虑 qiankun 这类基于路由的沙箱方案，或用模块联邦做运行时共享，代价是样式隔离、JS 沙箱逃逸、公共依赖版本冲突和调试复杂度。要点是定好主应用负责导航、登录与全局状态，子应用独立构建部署并注册路由，同时约定公共依赖版本、通信机制和统一的监控埋点。",
		Keywords:  []string{"微前端", "qiankun", "模块联邦", "沙箱", "样式隔离", "独立部署", "路由", "monorepo", "公共依赖", "通信"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimCommunication,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "前端状态管理你会怎么选？什么情况下不该引入 Redux 这类全局状态库？",
		Intent:    "考察技术选型的判断力与表达结构。",
		Reference: "先按状态的性质分类：只在一个组件内用的用 useState，跨层级但变化少的用 Context，表单用受控组件或表单库，来自服务端的数据优先交给请求缓存层（如 React Query/SWR）而不是全局 store。真正需要全局状态库的是多模块共享、需要中间件与时间旅行调试的大型应用。不该引入的场景是页面级状态简单、组件层级不深，此时全局库只是把复杂度从 props 挪到了 action 和 reducer。能说出取舍而不是只报库名，就符合预期。",
		Keywords:  []string{"状态管理", "redux", "zustand", "context", "单向数据流", "服务端状态", "react query", "局部状态", "不可变", "选型"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "SSR 应用出现首屏闪烁和 hydration mismatch 报错，你会怎么定位和解决？",
		Intent:    "考察同构渲染的排障能力。",
		Reference: "mismatch 的根因是服务端与客户端首次渲染的输出不一致，常见来源有 Date/时区、Math.random、window/localStorage 等只在客户端存在的值、用户鉴权后的差异化渲染、以及 HTML 结构非法（如 p 里套 div）。定位方法是看控制台报错的组件路径，并在服务端与客户端分别打印关键 props 对比。解决思路是让首屏渲染只依赖可序列化的确定性数据，把时间与随机值放到 useEffect 或挂载后再渲染，用 suppressHydrationWarning 只作为最后手段；首屏闪烁通常来自样式未内联或客户端二次请求，需内联关键 CSS 并把首屏数据随 HTML 一起下发。",
		Keywords:  []string{"ssr", "水合", "hydration", "mismatch", "首屏闪烁", "时区", "随机数", "useeffect", "序列化", "关键css"},
	},
	{
		Family:    FamilyFrontend,
		Dimension: DimDepth,
		Levels:    []string{LevelExpert},
		Question:  "React 18 的并发渲染解决了什么问题？useTransition 和 useDeferredValue 分别该在什么场景使用？",
		Intent:    "考察对并发特性原理与适用边界的理解。",
		Reference: "并发渲染把渲染变成可中断、可分片的工作，按优先级调度：高优先级更新（输入、点击）可以打断正在进行的低优先级渲染，避免长任务阻塞主线程导致输入卡顿。它靠 Fiber 链表结构与时间切片实现，配合 startTransition 把更新标记为过渡任务。useTransition 适合「由用户操作触发的、可以稍后完成的重渲染」，还能拿到 isPending 做加载态；useDeferredValue 适合「值本身来自 props 或状态、想让派生渲染延后」，不需要改动更新点。两者都不减少计算量，只是让紧急更新先落地，所以要能说清它们治的是卡顿不是耗时。",
		Keywords:  []string{"并发渲染", "fiber", "可中断", "时间切片", "usetransition", "usedeferredvalue", "优先级", "starttransition", "批处理", "ispending"},
	},

	// --- fullstack -------------------------------------------------------
	{
		Family:    FamilyFullstack,
		Dimension: DimCommunication,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "设计一个列表分页接口，你会如何定义请求参数和返回结构？为什么深分页推荐用游标而不是 offset？",
		Intent:    "考察接口契约设计与对深分页代价的认知。",
		Reference: "参数应包含分页游标或 page/pageSize、排序字段与方向、过滤条件，返回体给出 items、nextCursor/hasMore，必要时附 total（但 total 在大表上代价高，可用估算或异步统计）。offset 分页在深翻页时数据库要扫描并丢弃前 N 行，越翻越慢且并发插入会导致数据重复或漏读；游标分页带上一次最后一条的排序键（如 created_at + id），配合联合索引做范围查询，复杂度稳定为 O(1) 定位。还要提到排序键必须唯一或有兜底字段，否则游标位置不确定。",
		Keywords:  []string{"分页", "offset", "游标", "cursor", "nextcursor", "排序", "深分页", "联合索引", "total", "契约"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "Session 和 JWT 各有什么优缺点？在一个单体 Web 应用里你会选哪个，为什么？",
		Intent:    "考察对鉴权方案权衡的准确理解。",
		Reference: "Session 把状态存在服务端，Cookie 只带不透明的 sessionId，优点是能随时吊销、便于统计在线与踢下线，缺点是服务端要有存储、多实例需要共享会话（Redis）。JWT 把声明签在令牌里，服务端无状态、易横向扩展、适合跨服务传递，缺点是签发后难以撤销、载荷过大、需要额外机制（黑名单、短有效期 + 刷新令牌）处理注销与密钥轮换。单体应用通常选 Session + Redis 更简单安全，前端与后端同源时用 HttpOnly + SameSite Cookie 还能顺带缓解 XSS 窃取和 CSRF。",
		Keywords:  []string{"session", "jwt", "cookie", "httponly", "samesite", "无状态", "吊销", "刷新令牌", "redis", "csrf"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "用户反馈「保存失败」但服务端日志里没有明显报错，你会怎么从浏览器一路排查到数据库？",
		Intent:    "考察全链路排障的层次化思路。",
		Reference: "先复现并固定现场：确认浏览器版本、账号、操作路径，看 Network 里请求是否发出、状态码与响应体是什么，是被前端校验拦住、被网关拦截还是真的到了后端。若请求到后端，用 traceId 串起网关、应用与数据库日志，确认是否卡在参数校验、鉴权、事务回滚或下游超时；再查数据库慢查询、连接池耗尽与死锁日志。若请求根本没发出，问题在前端状态或浏览器插件/CSP。最后要沉淀为可观测性改进：统一请求 ID、结构化日志、前端错误上报，并把这次排查写成复盘的不变量。",
		Keywords:  []string{"复现", "network", "traceid", "结构化日志", "鉴权", "事务回滚", "连接池", "慢查询", "可观测性", "前端上报"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "实现大文件上传，如何支持分片、断点续传和秒传？",
		Intent:    "考察前后端配合的完整方案设计。",
		Reference: "前端用 File.slice 按固定大小分片，并发上传并限制并发数以便失败可重试；上传前先算文件哈希（可用抽样哈希或 Web Worker 计算以免卡住主线程），把哈希发给服务端查询是否已存在，存在即秒传。断点续传靠服务端记录已接收的分片索引，前端查询断点后从缺失分片继续。服务端按 uploadId 保存分片到临时目录或对象存储，全部分片到齐后按序合并并做整体校验，同时设置分片过期清理。要点是幂等（同一分片重传要覆盖而不是追加）、进度反馈与弱网重试。",
		Keywords:  []string{"分片", "断点续传", "秒传", "哈希", "并发上传", "合并", "重试", "对象存储", "进度", "幂等", "worker"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "ORM 的 N+1 查询问题是怎么产生的？你会怎么发现并优化它？",
		Intent:    "考察数据访问层的性能自觉。",
		Reference: "N+1 出现在「先查一批主记录，再在循环里逐条查关联」的写法上，一条列表接口就变成了 1+N 次数据库往返，连接池很快耗尽。发现方式是打开 SQL 日志或 APM 统计单次请求的 SQL 条数，压测时观察 QPS 与数据库连接数背离。优化手段：用预加载（preload/prefetch）把关联查询合并，或用 join 一次取回，或按主键批量 IN 查询后在内存组装，再配合缓存热点数据；同时要警惕预加载过度导致笛卡尔积膨胀，需要限制嵌套层级并分页。",
		Keywords:  []string{"n+1", "预加载", "join", "批量查询", "懒加载", "sql日志", "连接池", "缓存", "笛卡尔积", "apm"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "站内通知要实时推送给用户，SSE 和 WebSocket 你怎么选？",
		Intent:    "考察对实时通信技术边界的判断。",
		Reference: "SSE 基于 HTTP 单向推送、自动重连、实现简单、对网关和鉴权友好，适合「服务端到客户端」的通知、进度与流式输出，缺点是单向、每个连接占用一个 HTTP 长连接、在 HTTP/1.1 下有同域连接数限制。WebSocket 是全双工、适合聊天、协同编辑等双向高频场景，但需要额外处理心跳、重连、鉴权（握手阶段带 token）和浏览器兼容与网关升级（Nginx 需配置 Upgrade）。选型结论应给出降级方案：实时通道不可用时退回轮询或拉取，保证功能不丢。",
		Keywords:  []string{"sse", "websocket", "长连接", "心跳", "重连", "单向推送", "全双工", "nginx", "降级轮询", "鉴权"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimAccuracy,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "一次完整的 CI/CD 你会怎么设计？灰度发布和快速回滚的关键点是什么？",
		Intent:    "考察交付链路与变更安全的整体设计。",
		Reference: "流水线按阶段来：提交触发静态检查与单元测试，构建产出不可变制品（镜像或二进制，带 commit 与版本号），部署到测试环境跑集成测试，再走生产发布并保留审批与变更记录。配置与代码分离，密钥走密钥管理而不是写进仓库。灰度发布按用户、地域或流量比例逐步放量，发布前必须配好就绪探针与优雅停机，观察黄金指标（延迟、错误率、饱和度）自动判定推进或中止；回滚要能在分钟级完成，因此要保证制品可追溯、数据库变更向前兼容（先加列后改代码），避免只回滚代码导致结构不匹配。",
		Keywords:  []string{"ci/cd", "流水线", "制品", "灰度", "蓝绿", "回滚", "就绪探针", "健康检查", "配置管理", "黄金指标", "向前兼容"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "单体的包越来越大、发布互相阻塞，你会如何拆服务？拆之前必须先做什么？",
		Intent:    "考察架构演进的节奏感与边界判断。",
		Reference: "先不要动服务，先做模块化：整理领域边界与依赖方向，用包级接口和依赖注入隔断循环依赖，把共享代码沉淀为库，这一步能立刻降低发布耦合。同时补齐可观测性与接口契约，否则拆完出了问题无法定位。拆分时优先选变更频率高、资源需求差异大或可用性要求不同的边界，先做读写分离或独立部署模块，再考虑独立进程；随后要接受分布式事务、跨服务查询和运维成本上升，用异步消息与最终一致性换取独立性。",
		Keywords:  []string{"模块化", "领域边界", "依赖倒置", "契约", "服务拆分", "可观测性", "分布式事务", "最终一致性", "灰度", "成本"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "XSS 和 CSRF 在前后端分别怎么防？为什么 HttpOnly 无法覆盖 XSS 的全部危害？",
		Intent:    "考察 Web 安全的理解深度而非清单式背诵。",
		Reference: "XSS 要靠输出编码：按上下文做 HTML/属性/JS 转义，富文本用白名单过滤，配合 CSP 限制内联脚本与外部来源，Cookie 加 HttpOnly 防止脚本读取会话。CSRF 的核心是「浏览器自动带 Cookie」，防护靠 SameSite、CSRF Token（校验来源并防止被读取）、检查 Origin/Referer，以及对状态变更接口强制 POST。HttpOnly 只保护 Cookie 不被 JS 读取，攻击者仍可在页面内以用户身份发起请求、篡改 DOM、窃取页面上的敏感数据或调用同源接口，所以 XSS 的危害远大于会话窃取，还必须防越权与最小权限。",
		Keywords:  []string{"xss", "csrf", "转义", "csp", "httponly", "samesite", "token", "输入校验", "越权", "sql注入", "referer"},
	},
	{
		Family:    FamilyFullstack,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "前端页面为什么不直连数据库？一次请求从浏览器到数据库要经过哪些环节？",
		Intent:    "考察对分层架构与信任边界的理解。",
		Reference: "直连数据库意味着把凭证和库结构暴露给浏览器，任何用户都能绕过业务规则、越权改数据，也无法做统一的审计、限流和事务编排。正常链路是浏览器发起请求，经 DNS 与 TLS 到达反向代理或网关（负责 TLS、路由、鉴权、限流），进入应用服务做参数校验、业务逻辑与事务控制，通过连接池访问数据库；中间还可能经过缓存、消息队列与下游服务。能指出每一层各自拦掉什么问题（安全、格式、并发、一致性），就说明理解了分层的价值。",
		Keywords:  []string{"同源", "鉴权", "凭证泄漏", "nginx", "网关", "参数校验", "事务", "连接池", "缓存", "限流", "分层"},
	},

	// --- data ------------------------------------------------------------
	{
		Family:    FamilyData,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "用 SQL 求每个部门薪资第二高的员工，你会怎么写？窗口函数相比 GROUP BY 的优势是什么？",
		Intent:    "考察 SQL 基本功与对窗口函数的理解。",
		Reference: "标准写法是用窗口函数：row_number() over (partition by dept_id order by salary desc) 得到部门内排名，外层过滤 rn = 2；若要处理并列薪资，则用 dense_rank 而不是 row_number。GROUP BY 是聚合，会把多行折叠成一行，无法保留明细行；窗口函数在保留原始行的同时附加聚合或排名结果，适合排名、累计、同环比这类分析。实际写的时候要注意并列、并列后续名次、部门人数不足时返回空，以及在大数据引擎里窗口的 shuffle 代价。",
		Keywords:  []string{"窗口函数", "row_number", "dense_rank", "partition by", "group by", "order by", "排名", "并列", "shuffle", "子查询"},
	},
	{
		Family:    FamilyData,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "数仓为什么要分层？ODS、DWD、DWS、ADS 各自承担什么职责？",
		Intent:    "考察数仓方法论是否成体系。",
		Reference: "分层的价值是把「贴源」和「面向业务」解耦：变更被隔离在底层，上层口径稳定，任务可复用且便于定位问题。ODS 几乎原样落库并保留变更，DWD 做清洗、去重、维度退化与一致性维度对齐，保留明细，DWS 按主题和粒度做轻度汇总（如用户日粒度行为），ADS 面向报表与接口做最终加工。回答能提到命名规范、分区与生命周期、血缘和调度依赖，以及避免跨层反向依赖，就比单纯背定义更有说服力。",
		Keywords:  []string{"数仓分层", "ods", "dwd", "dws", "ads", "明细", "汇总", "血缘", "口径统一", "调度依赖", "生命周期"},
	},
	{
		Family:    FamilyData,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "一个 Hive 或 Spark 任务跑了三小时还没结束，你会怎么定位？数据倾斜怎么解？",
		Intent:    "考察大数据任务的诊断与调优能力。",
		Reference: "先看作业详情定位卡点：是某个 stage 的 reduce 长时间不动，还是输入数据量暴涨，或资源排队。看任务的反压与倾斜图，若少数 task 处理的数据量远高于其他，就是热点 key 造成的数据倾斜。解法分两类：能 map 端处理的用 map join 广播小表；必须 reduce 端聚合的，对热点 key 加随机盐打散后两阶段聚合，或对空值/null key 单独处理，也可提高并行度、调整分区数与文件大小合并小文件。最后要确认瓶颈是否在 shuffle 数据量而不是算力，避免盲目加资源。",
		Keywords:  []string{"数据倾斜", "热点key", "加盐", "两阶段聚合", "mapjoin", "并行度", "小文件", "shuffle", "oom", "stage", "反压"},
	},
	{
		Family:    FamilyData,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "昨天日活突然掉了 15%，作为数据分析师你会怎么排查并给出结论？",
		Intent:    "考察指标异动的结构化归因能力。",
		Reference: "先确认是真跌还是数据问题：检查埋点上报延迟、任务是否失败、口径是否变更、是否有跨天补数，这一步能排掉大部分「假跌」。确认后按维度下钻拆分：分渠道、地域、机型、版本、新老用户，看跌幅集中在哪个子群；再看漏斗各环节转化率，判断是入口曝光下降、登录失败还是页面报错。找到具体子群后结合发版、运营活动、竞品动作提出假设并验证。结论要给出量级、范围、根因与建议动作，而不是只报一个数字。",
		Keywords:  []string{"数据质量", "埋点", "上报延迟", "口径", "维度下钻", "漏斗", "同比", "环比", "分群", "归因", "假设验证"},
	},
	{
		Family:    FamilyData,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "AB 实验你会怎么设计？样本量、分流和显著性分别要注意什么？",
		Intent:    "考察实验设计能力，避免把相关当因果。",
		Reference: "先定唯一的核心指标与护栏指标，并写清假设与预期最小提升幅度（MDE），据此反推所需样本量与实验周期，避免「没显著就再等几天」的偷看偏差。分流要对用户维度做哈希保证同一用户稳定进组，且两组在地区、设备、活跃度上均衡，上线前用 AA 测试验证分流无偏。分析时看 p 值与置信区间，注意多重比较、新奇效应和指标间的相互影响；如果实验期间有发版或大促，要能识别干扰。结论要区分统计显著与业务显著，必要时延长周期或做分层分析。",
		Keywords:  []string{"ab测试", "分流", "样本量", "置信区间", "p值", "显著性", "aa测试", "核心指标", "护栏指标", "新奇效应", "周期"},
	},
	{
		Family:    FamilyData,
		Dimension: DimCommunication,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "留存率和漏斗分析分别能回答什么问题？你会怎么向业务方讲清楚指标口径？",
		Intent:    "考察指标理解与对外沟通能力。",
		Reference: "留存率回答「用户会不会回来」：按同期群（cohort）看次日、7 日、30 日留存，衡量产品的长期价值与黏性，也用于判断渠道质量。漏斗回答「用户在每一步流失多少」：把关键路径拆成有序步骤，看各步转化率与绝对流失量，用于定位体验断点。给业务方讲口径时要明确三件事：分母是什么（新增还是活跃、是否去重）、时间窗口怎么算（自然日还是滚动 24 小时）、异常怎么处理（多端登录、测试账号）。能做出一张口径卡（定义 + 计算 SQL + 负责人）就说明具备工程化的数据意识。",
		Keywords:  []string{"留存", "同期群", "漏斗", "转化率", "口径", "去重", "时间窗口", "北极星指标", "cohort", "维度"},
	},
	{
		Family:    FamilyData,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "实时数仓的 Lambda 和 Kappa 架构怎么取舍？Flink 的水位线和精确一次是怎么实现的？",
		Intent:    "考察实时链路的架构判断与原理掌握。",
		Reference: "Lambda 保留批处理保证准确、流处理保证时效，代价是两套代码两条链路容易口径不一致；Kappa 只保留一条流式链路，用重放历史消息处理回溯，代价是消息保留成本与流计算的高要求。Flink 的水位线是「时间不超过某点的数据已到齐」的进度信号，用于触发窗口计算，配合 allowed lateness 处理迟到数据；精确一次靠 checkpoint 做状态快照 + 数据源可重放（Kafka offset 与状态同批提交），下游幂等或两阶段提交保证端到端。还要能说出背压、状态后端选型与乱序容忍的取舍。",
		Keywords:  []string{"lambda", "kappa", "flink", "水位线", "watermark", "checkpoint", "精确一次", "状态后端", "乱序", "kafka", "背压"},
	},
	{
		Family:    FamilyData,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "出现过核心指标算错的数据质量事故，你会建立哪些防线来防止复发？",
		Intent:    "考察数据治理的体系化落地能力。",
		Reference: "防线分三层：事前把口径固化成指标字典与公共模型，变更走评审与影响分析（依赖血缘找出受影响的报表）；事中用稽核规则做监控——行数波动、主键唯一、非空、枚举合法、金额对账、与上游数据比对，并对关键任务设置 SLA 告警与重跑策略；事后能快速止血：锁定受影响产出并回滚到上一个正确分区，对外同步影响范围和时间线，复盘根因并补充测试用例。要点是承认数据出错是必然的，关键是把发现时间从「业务发现」提前到「监控发现」。",
		Keywords:  []string{"数据质量", "稽核", "监控告警", "基线", "对账", "血缘", "影响分析", "回滚", "sla", "指标字典", "复盘"},
	},
	{
		Family:    FamilyData,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "设计一个用户标签画像系统，标签如何组织和更新？人群圈选怎么做得快？",
		Intent:    "考察画像系统的建模与工程权衡。",
		Reference: "标签分三类：统计类（近 30 天消费额）、规则类（是否高价值）、模型类（预测流失概率）；按更新频率又分离线 T+1 与实时标签。存储通常用宽表（Hive/ClickHouse 做明细与聚合）+ 标签字典（标签 ID、口径、负责人、更新周期）+ 位图索引（RoaringBitmap 存每个标签命中的用户集合），人群圈选即对位图做交并差，能在秒级返回结果和规模。更新上离线走调度、实时走 Flink 写 KV 或列存；要注意标签口径版本化、用户 ID 映射（设备号与账号打通）和隐私合规。",
		Keywords:  []string{"用户画像", "标签", "位图", "roaringbitmap", "人群圈选", "宽表", "离线", "实时", "更新频率", "id映射", "标签字典"},
	},
	{
		Family:    FamilyData,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "一份上万行的数据用 pandas 处理要几十分钟，你会怎么优化？",
		Intent:    "考察数据分析脚本的工程化能力。",
		Reference: "先量化再优化：用 %%time 或 profiler 找到最慢的一步，通常是 apply 逐行、iterrows 或字符串操作。优化手段包括向量化（用内置函数替代 apply）、选对 dtype（category、int32、避免 object）、把多次过滤合并、用 query/eval、避免链式赋值与拷贝、必要时分块读取或下推到数据库执行。如果数据量超出单机内存，应换工具而不是硬调 pandas：用 Polars/DuckDB 单机处理，或把聚合下推到 SQL、Spark。最后要留下可复现的脚本与结论，而不是只跑一次 notebook。",
		Keywords:  []string{"pandas", "向量化", "apply", "dtype", "category", "分块", "内存", "profiler", "duckdb", "sql下推", "拷贝"},
	},
	{
		Family:    FamilyData,
		Dimension: DimAccuracy,
		Levels:    []string{LevelExpert},
		Question:  "维度建模里星型和雪花模型如何取舍？缓慢变化维你选哪种实现？",
		Intent:    "考察维度建模的理论与实践判断。",
		Reference: "星型把维度退化到一张宽表，查询 join 少、性能好、对 BI 友好，代价是维表冗余与更新成本；雪花做规范化拆分，节省存储、便于维护层级，但查询要多层 join，复杂度和出错率上升。多数分析场景优先星型，只有维度极宽或层级需要独立管理时才规范化。缓慢变化维：类型 1 直接覆盖（丢弃历史）、类型 2 用生效/失效时间和版本号新增行（保留历史，最常用）、类型 3 加前值列保留有限历史；大数据场景常用拉链表实现类型 2，要讲清代理键、分区与末次状态查询的写法。",
		Keywords:  []string{"维度建模", "星型", "雪花", "事实表", "维度表", "scd", "拉链表", "代理键", "一致性维度", "生效时间", "版本号"},
	},

	// --- algorithm -------------------------------------------------------
	{
		Family:    FamilyAlgorithm,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "请手写一个 LRU 缓存，说明你为什么选这种数据结构，以及各个操作的时间复杂度。",
		Intent:    "考察基础数据结构组合与复杂度分析。",
		Reference: "标准解法是哈希表 + 双向链表：哈希表 O(1) 定位节点，双向链表维护访问顺序，头部是最近使用、尾部淘汰，因此 get 与 put 都是 O(1)。要说明为什么用双向链表而不是单链表——删除任意节点需要前驱指针；为什么不用切片——移动元素是 O(n)。实现细节包括容量为 0 或 1 的边界、put 已存在 key 要更新值并移到头部、以及用哨兵头尾节点简化空链表判断。加分项是提到并发场景要用分段锁或 sync.Map 加锁粒度。",
		Keywords:  []string{"lru", "哈希表", "双向链表", "o(1)", "淘汰", "哨兵节点", "并发", "锁", "ttl", "容量", "边界"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "快速排序和归并排序在稳定性、最坏复杂度和工程应用上有什么区别？",
		Intent:    "考察排序算法的准确理解与工程视角。",
		Reference: "快排平均 O(n log n)、最坏 O(n^2)（已排序加固定基准），原地排序、常数小、缓存友好，但不稳定；归并稳定且最坏也是 O(n log n)，但需要 O(n) 额外空间。工程上通用排序（如 Go 的 sort 与 Java 的 TimSort/双轴快排）会做混合与优化：小数组切插入排序、随机化或三数取中选基准、检测深度退化。能提到外部排序（数据大于内存时分块排序再归并）和时间/空间/稳定性的取舍，就比背复杂度更有价值。",
		Keywords:  []string{"快排", "归并", "稳定性", "时间复杂度", "最坏情况", "原地排序", "空间复杂度", "三数取中", "插入排序", "外部排序"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "动态规划题你会怎么定义状态和转移方程？请举一道你做过的题说明。",
		Intent:    "考察把问题抽象成状态转移的能力。",
		Reference: "方法论是四步：明确状态的物理含义（以 i 结尾/前 i 个的最优值）、写出转移方程并列出所有决策、确定初始条件与非法状态、确定遍历顺序并考虑空间压缩。以「最长递增子序列」为例，dp[i] 表示以 nums[i] 结尾的最长长度，转移是 dp[i] = max(dp[j]) + 1 且 nums[j] < nums[i]，复杂度 O(n^2)，可用贪心 + 二分优化到 O(n log n)。以「背包」为例要讲清 0-1 与完全背包遍历方向相反的根因。能主动说明为什么要用一维数组压缩以及从后往前遍历的原因，说明是真理解。",
		Keywords:  []string{"动态规划", "状态定义", "转移方程", "边界条件", "遍历顺序", "空间压缩", "记忆化", "最优子结构", "重叠子问题", "最长递增子序列", "背包"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "模型过拟合怎么判断、怎么缓解？正则化和交叉验证分别起什么作用？",
		Intent:    "考察机器学习基础概念的正确使用。",
		Reference: "过拟合的表现是训练集指标远好于验证集，且随训练继续验证指标变差。缓解手段包括增加数据与数据增强、降低模型复杂度、早停、正则化、Dropout、集成与 BatchNorm；其中 L1 倾向产生稀疏解可用于特征选择，L2 抑制权重幅度使解更平滑，两者都要通过验证集调系数。交叉验证解决的是「单次划分评估不稳定」的问题：K 折让每个样本都参与验证，得到更可靠的泛化估计，也用于选超参；但要注意时序数据不能随机划分，分组数据要按组划分，否则会数据泄漏。",
		Keywords:  []string{"过拟合", "偏差方差", "正则化", "l1", "l2", "交叉验证", "早停", "数据增强", "dropout", "数据泄漏", "泛化"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "特征工程里类别特征和高基数特征你分别怎么处理？",
		Intent:    "考察特征处理的实战经验。",
		Reference: "低基数列用 one-hot 或直接作为类别特征交给 GBDT；高基数列（用户 ID、商品 ID）用目标编码（必须用折外或平滑避免泄漏）、频次编码、哈希分桶，或直接学 embedding 交给深度模型。树模型对单调变换不敏感，因此归一化主要影响线性模型和神经网络；缺失值要区分「真缺失」与「值为零」，可用缺失指示特征保留信息。数值特征可做分桶获取非线性，交叉特征要有选择地构造并评估增益，否则维度爆炸且过拟合。最后强调所有统计量必须只用训练集计算。",
		Keywords:  []string{"特征工程", "one-hot", "target encoding", "embedding", "哈希分桶", "归一化", "高基数", "特征交叉", "缺失值", "分桶", "数据泄漏"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimAccuracy,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "样本极度不平衡时准确率为什么失效？你会用什么指标和手段？",
		Intent:    "考察对评估指标选择的理解。",
		Reference: "因为只要全预测为多数类就能拿到很高的准确率，模型其实没学到判别能力。应改用精确率/召回率、F1、AUC 与 PR 曲线，其中 PR 曲线对不平衡更敏感、AUC 在极不平衡时会被大量真负例「稀释」显得乐观。手段包括重采样（过采样 SMOTE 要小心合成样本的合理性、欠采样会丢信息）、类别权重或代价敏感学习、调整决策阈值以匹配业务对精确率与召回率的偏好，以及把问题重构成异常检测。最后要用与业务对齐的指标（如漏放率、打扰率）验收。",
		Keywords:  []string{"类别不平衡", "准确率失效", "auc", "pr曲线", "f1", "召回率", "精确率", "重采样", "smote", "类别权重", "阈值"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "梯度消失和梯度爆炸的成因是什么？批归一化和残差连接分别怎么缓解？",
		Intent:    "考察深度学习训练问题的原理级理解。",
		Reference: "反向传播是链式法则连乘，深层网络中雅可比矩阵的谱范数反复小于 1 就会梯度消失、反复大于 1 就会爆炸；饱和激活函数（sigmoid/tanh）的导数小于 1 会加剧消失。缓解手段：换 ReLU 及其变体、合理初始化（Xavier/He）、批归一化把每层输入标准化使激活分布稳定、残差连接给梯度提供恒等通路、梯度裁剪抑制爆炸、配合学习率 warmup。BatchNorm 在训练用批统计、推理用滑动平均，因此小 batch 或序列任务要考虑 LayerNorm/GroupNorm。",
		Keywords:  []string{"梯度消失", "梯度爆炸", "batchnorm", "残差连接", "激活函数", "relu", "权重初始化", "学习率", "梯度裁剪", "layernorm", "链式法则"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimDepth,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "Transformer 的自注意力是怎么计算的？为什么要除以根号 dk，并且要用多头？",
		Intent:    "考察对注意力机制公式背后动机的理解。",
		Reference: "把输入映射成 Q、K、V 三个矩阵，用 Q 与 K 的点积得到相似度，除以根号 dk 后做 softmax 归一化成权重，再对 V 加权求和；除以根号 dk 是因为维度增大时点积方差随 dk 线性增长，softmax 会进入饱和区导致梯度极小。多头把特征维度切成若干子空间独立做注意力，让模型同时关注不同位置关系与语义子空间，最后拼接再线性变换。因为自注意力本身无位置概念，必须加位置编码；复杂度是序列长度的平方，因此长序列要用稀疏、滑窗或线性注意力优化。",
		Keywords:  []string{"attention", "qkv", "self-attention", "多头", "缩放", "softmax", "位置编码", "复杂度", "transformer", "并行", "饱和"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "推荐系统从召回、粗排到精排，各阶段的目标和常用模型分别是什么？",
		Intent:    "考察推荐链路的分层设计与取舍。",
		Reference: "召回从百万级候选中快速筛出数百到数千个，目标是高召回、低延迟，常用多路召回：协同过滤（itemcf/usercf）、双塔向量检索、热度与规则兜底，用于解决冷启动与多样性。粗排对几百到几千条做轻量打分，平衡效果与算力，常用双塔或浅层模型。精排用特征丰富的复杂模型（如 DeepFM/DIN 这类 CTR 模型）精确预估，再做重排引入多样性、打散与业务约束（如去重、广告位）。要讲清各阶段的样本、特征与目标一致性，以及线上延迟预算如何倒推模型复杂度。",
		Keywords:  []string{"召回", "粗排", "精排", "重排", "协同过滤", "双塔", "embedding", "ctr", "特征", "多样性", "冷启动", "延迟"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelExpert},
		Question:  "一个模型离线指标很好但线上效果差，你会从哪些方面排查？",
		Intent:    "考察算法工程化与线上线下一致性的认知。",
		Reference: "先查数据链路：特征穿越（用了未来信息）、训练与推理特征口径不一致、特征服务返回默认值或延迟导致的陈旧特征，这是最常见的一类。再查样本：离线样本的分布与线上真实流量存在选择偏差，负样本采样方式改变了先验。然后查目标与评估：离线指标与业务指标不一致、评估集泄漏。还要查工程：模型版本上线错、打分超时被降级、阈值与分档不匹配、实验分流不干净。排查手段是做特征对齐校验、影子模式对比离线打分与线上打分、以及干净的 AB 实验。",
		Keywords:  []string{"特征穿越", "线上线下不一致", "样本偏差", "特征服务", "延迟", "数据泄漏", "影子模式", "ab测试", "监控", "降级", "版本"},
	},
	{
		Family:    FamilyAlgorithm,
		Dimension: DimCommunication,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "说说你理解的机器学习和传统规则系统的区别，为什么很多场景仍然要用规则？",
		Intent:    "考察工程判断力与结构化表达。",
		Reference: "规则是人写死的判断逻辑，可解释、可控、上线快、零样本也能跑，但难覆盖长尾、维护成本随场景线性增长；机器学习从数据中学到泛化模式，能处理高维非线性问题，但依赖样本质量、可解释性弱、冷启动困难。所以实际系统通常是混合：用规则做兜底、风控红线、冷启动和快速修复，用模型做主流量排序与预测。好的回答会给出一个自己项目里的例子，说明哪部分用了规则、为什么，以及怎么评估两部分的收益。",
		Keywords:  []string{"机器学习", "规则", "可解释性", "冷启动", "泛化", "样本", "兜底", "风控", "混合", "成本", "长尾"},
	},

	// --- product ---------------------------------------------------------
	{
		Family:    FamilyProduct,
		Dimension: DimLogic,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "手上有一堆需求要做，你怎么排优先级？说一个你实际用过的判断框架。",
		Intent:    "考察优先级判断的框架化思维。",
		Reference: "先说清前提：优先级服务于目标，所以先对齐本季度最重要的业务目标，再谈排序。常用框架有 KANO（区分基本型、期望型、兴奋型需求）、RICE（触达人数 × 影响 × 置信度 ÷ 成本）、以及紧急重要矩阵。实际使用时要敢于把「老板想要」和「用户需要」分开评估，并把结论与依据写下来让相关方看见。回答能指出框架只是沟通工具、真正的难点是估准影响与成本，并提到为高价值需求预留缓冲，会更有说服力。",
		Keywords:  []string{"优先级", "rice", "kano", "roi", "业务目标", "成本", "紧急重要", "用户反馈", "置信度", "判断框架"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimSituational,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "研发说这个需求做不了，或者工期要翻倍，你会怎么推进？",
		Intent:    "考察资源受限下的协调与取舍能力。",
		Reference: "先弄清「做不了」的真实原因：是技术不可行、工作量被低估，还是方案设计过重。然后回到目标上重新裁剪：能否做 MVP 只保留核心路径，能否分期上线，能否用配置或运营手段替代开发。同时要让研发给出工作量的拆分依据，一起看哪些部分可并行或延后。若确实无法按期，就要向上同步风险并给出选项（延期、减范围、加人），而不是私下把问题拖到截止日。整个过程要留下书面结论，避免事后扯皮。",
		Keywords:  []string{"方案裁剪", "mvp", "工作量拆分", "技术方案", "风险同步", "取舍", "排期", "对齐目标", "分期上线", "沟通"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimLogic,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "核心指标连续三天下滑，你会怎么定位原因并给出结论？",
		Intent:    "考察数据驱动的假设验证能力。",
		Reference: "第一步排除数据问题：埋点、上报延迟、口径变更、统计任务失败，避免对着假数据行动。第二步拆结构：按公式把指标拆成因子（如订单量 = 活跃用户 × 下单率 × 人均单数），再按渠道、地区、版本、新老用户各维度下钻，找到贡献最大的子群。第三步提假设并验证：是否与发版、运营活动、竞品动作、系统故障的时间点吻合，必要时做小范围对照。最后给出结论时要说清影响量级、是否已止损、以及需要谁做什么，而不是只丢一个归因图。",
		Keywords:  []string{"指标拆解", "维度下钻", "假设验证", "数据质量", "口径", "归因", "发版", "对照", "影响量级", "止损"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "请从 0 到 1 设计一个面向中小企业的报销功能，你会怎么开始？",
		Intent:    "考察需求梳理与方案落地的完整度。",
		Reference: "先明确用户与场景：谁是提交人、谁是审批人、谁是财务，各自在什么设备上完成什么动作；再通过访谈和现有流程梳理找出真正的痛点（比如贴票、找领导签字、月底对账）。接着定义核心流程与边界（金额分档审批、预算校验、发票验真、与财务系统对接），画出流程图并定义异常分支（驳回、撤回、跨月）。然后做优先级分期：一期只做提交 + 审批 + 导出，二期做验真与自动入账。最后给出可度量的成功标准，例如人均报销耗时、驳回率、财务对账工时。",
		Keywords:  []string{"用户访谈", "场景", "痛点", "流程图", "异常分支", "需求文档", "原型", "mvp", "验收标准", "成功指标", "分期"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimFit,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "你做过最成功的一个需求是什么？你怎么衡量它的价值？",
		Intent:    "考察价值意识与自我评估的客观度。",
		Reference: "好回答会按背景—目标—决策—结果来讲：说明当时的业务问题、你定下的可量化目标、你在方案上有过哪些取舍、上线后数据发生了什么变化。衡量价值要区分直接指标（转化率、使用率、留存）与间接收益（客服工单下降、运营效率提升），并说明是自然增长还是你的改动带来的（对照或灰度）。同时要诚实说出没达成的部分和原因，这比只讲成绩更能体现判断力。",
		Keywords:  []string{"背景", "目标", "北极星指标", "数据结果", "灰度", "对照", "迭代", "复盘", "价值", "用户反馈"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimCommunication,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "用户访谈里怎么区分「用户说的」和「用户真正需要的」？",
		Intent:    "考察需求辨别能力与提问技巧。",
		Reference: "核心是问行为而不是问意愿：追问最近一次的真实经历、当时怎么做的、为什么这么做，而不是问「如果有这个功能你会用吗」。要警惕诱导性提问和样本偏差（只访问活跃用户），也不能把单个尖锐反馈当成普遍需求。拿到原声后回到场景中还原动机，再做成可验证的假设，用数据或小流量实验确认规模与优先级。落地时可以沉淀一张「用户原声 → 场景 → 假设 → 验证方式」的表格。",
		Keywords:  []string{"用户访谈", "伪需求", "场景还原", "动机", "用户原声", "样本偏差", "诱导性提问", "假设", "验证", "小流量"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimSituational,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "功能上线两周数据不达预期，老板要你给个说法，你会怎么处理？",
		Intent:    "考察面对预期落差时的担当与复盘能力。",
		Reference: "先给事实再给判断：上线后的真实数据、与预期目标的差距、以及差距出在漏斗的哪一环（曝光、进入、完成、复购）。然后区分三类原因：需求假设错了（用户其实没这个痛点）、方案没做好（路径太长、提示不清）、分发不够（入口曝光不足）。针对可修复的原因给出迭代计划和时间点，对不成立的假设也要明确说「应该止损」，而不是继续投入。沟通时避免甩锅给研发或运营，同时把这次判断偏差沉淀成后续立项的检查项。",
		Keywords:  []string{"复盘", "预期差", "漏斗", "假设验证", "留存", "灰度", "迭代", "止损", "决策", "立项检查"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimTeamwork,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "同时对接设计、研发、测试和运营，你怎么保证信息不丢、节奏不乱？",
		Intent:    "考察跨职能协作的组织能力。",
		Reference: "关键是让目标、范围、时间和责任人始终唯一且可见：立项时把目标和范围写成文档并让各方确认，排期时明确里程碑与依赖关系。节奏上靠固定同步（周会/站会）加异步文档：变更必须落到文档并广播给受影响方，避免只在群里口头说。风险要提前暴露并有 owner，交付后做一次简短复盘。还要注意向上管理——让老板知道进度与风险；也要避免自己成为唯一的信息中转站，把关键结论沉淀成团队共享的知识。",
		Keywords:  []string{"跨部门", "目标对齐", "里程碑", "风险同步", "责任人", "文档沉淀", "异步沟通", "变更管理", "闭环", "复盘"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimLogic,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "要做一个会员付费功能，你会怎么设计权益、定价和增长路径？",
		Intent:    "考察商业化设计的结构化思考。",
		Reference: "先定义付费动机：用户为什么现在就要付钱，权益必须对应高频且可感知的价值（省时间、省钱、专属内容），而不是把原有功能锁起来逼付费。定价上要参考竞品与用户支付意愿，做分层（连续包月、季卡、年卡）并用首月优惠降低决策门槛，同时算清 LTV 与获客成本的关系。增长路径围绕转化漏斗设计：试用权益、到期提醒、续费优惠、老带新；每一步都要定义指标并做 AB 实验验证，避免一次上线大改无从归因。",
		Keywords:  []string{"会员权益", "定价", "支付意愿", "分层", "ltv", "arpu", "付费转化", "到期提醒", "续费", "ab实验", "漏斗"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimSelfAwareness,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "讲一个你参与过的失败项目，你从中总结出了什么，后来怎么改进的？",
		Intent:    "考察复盘深度与自我认知。",
		Reference: "好的复盘会落到可控因素上：比如立项时没有验证核心假设、需求范围失控、缺少上线后的数据监控，而不是归因于「老板拍脑袋」或「研发不配合」。要说明你当时的具体判断和依据，事后回看是哪一步错了，以及你为此改变了什么做法——例如现在会先做小流量验证、会在需求评审时就定义验收指标、会主动做风险清单。能说出改进措施后来被验证有效（有结果），比单纯认错更有分量。",
		Keywords:  []string{"复盘", "根因", "核心假设", "范围失控", "验收指标", "小流量", "风险清单", "改进", "教训", "反思"},
	},
	{
		Family:    FamilyProduct,
		Dimension: DimCommunication,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "当数据和你的直觉冲突时，你会怎么做决策？请举一个例子。",
		Intent:    "考察数据素养与决策的成熟度。",
		Reference: "先质疑数据而不是质疑直觉：检查口径、样本、时间窗口，确认差异不是统计噪音或数据质量问题。若数据可靠，就把直觉当作一个待验证的假设，设计小成本实验（小流量灰度或 AB）去证伪，而不是靠职位压过数据。也要承认有些决策无法实验（如战略方向、品牌投入），此时要靠清晰的判断依据和可回退的方案降低风险。例子里要讲清你的直觉来源、如何被数据修正、最终结果如何，体现既尊重数据也不迷信数据。",
		Keywords:  []string{"数据", "直觉", "假设", "ab实验", "小流量", "灰度", "口径", "证伪", "可回退", "决策", "依据"},
	},

	// --- ops -------------------------------------------------------------
	{
		Family:    FamilyOps,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "K8s 的 liveness、readiness 和 startup 探针分别解决什么问题？配错会有什么后果？",
		Intent:    "考察容器编排基础与故障直觉。",
		Reference: "readiness 决定 Pod 是否加入 Service 接收流量，失败只会被摘掉而不会重启，用于依赖未就绪或启动慢的场景；liveness 失败会触发容器重启，用于进程假死，配得太激进（超时短、失败阈值低）会在高负载下误杀导致雪崩；startup 探针保护启动慢的应用，在它成功前禁用其他探针，避免启动期被 liveness 杀掉。三者都要配好 initialDelay、period、timeout 与 failureThreshold，并和优雅停机（preStop + terminationGracePeriodSeconds）配合，否则滚动发布会丢请求。",
		Keywords:  []string{"k8s", "liveness", "readiness", "startup", "探针", "重启", "摘流量", "优雅停机", "滚动更新", "阈值", "依赖"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "容器的 request 和 limit 有什么区别？Pod 出现 OOMKilled 一般怎么排查？",
		Intent:    "考察资源模型与内存问题的定位路径。",
		Reference: "request 是调度依据，决定 Pod 被放到哪个节点以及 QoS 档位；limit 是运行上限，超过 CPU limit 会被节流（throttle），超过内存 limit 会被 OOMKill。排查 OOMKilled 要看 kubectl describe 的 Last State 与退出码 137，确认是容器被 cgroup 杀还是节点内存压力驱逐；再看容器内是否堆外内存（元空间、直接内存、线程栈）增长而堆监控看不出来、是否有内存泄漏或缓存无上限、limit 是否设得低于应用实际峰值。处置是修正 limit/request、加内存监控与 GC 日志，而不是简单调大。",
		Keywords:  []string{"request", "limit", "oomkilled", "退出码137", "cgroup", "节流", "内存泄漏", "堆外内存", "jvm", "监控", "驱逐"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "线上接口大面积超时，你会按什么顺序排查？",
		Intent:    "考察故障响应的优先级与分层定位能力。",
		Reference: "先止血再定位：能回滚就回滚，能扩容/限流就先执行，避免用户持续受影响，同时同步故障通告。定位时自上而下看黄金指标（延迟、错误率、流量、饱和度），用链路追踪找出耗时集中在哪个服务与哪个下游依赖，再进到该服务看线程/协程堆积、GC、连接池耗尽、慢 SQL、下游超时与重试风暴。还要检查是不是外部依赖或网络抖动、是不是缓存失效导致回源打满、是不是发布引入了变更。事后补监控与告警阈值，把这个故障模式变成可提前发现的信号。",
		Keywords:  []string{"止血", "回滚", "限流", "扩容", "黄金指标", "链路追踪", "连接池", "慢sql", "重试风暴", "缓存失效", "复盘"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "监控体系你会怎么搭？Prometheus 的四种指标类型分别适合什么场景？",
		Intent:    "考察可观测性建设的体系化认识。",
		Reference: "指标体系按 USE（利用率、饱和度、错误）看资源，按黄金指标（延迟、流量、错误、饱和度）看服务，再加业务指标与 SLO 错误预算；日志、指标、链路追踪三者互补，靠统一 traceId 与标签关联。Prometheus 的 Counter 只增不减适合请求数、错误数，用 rate/increase 计算速率；Gauge 可增可减适合内存、队列长度；Histogram 服务端分桶，可聚合算分位数；Summary 客户端算分位数，不能跨实例聚合。告警要基于症状和 SLO 而不是每个原因都报，避免告警疲劳。",
		Keywords:  []string{"prometheus", "counter", "gauge", "histogram", "summary", "exporter", "grafana", "黄金指标", "slo", "错误预算", "告警"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "用户报「网站打不开」，你怎么一层层定位是 DNS、网络、网关还是应用的问题？",
		Intent:    "考察分层排障的条理与工具熟练度。",
		Reference: "从外到内逐层验证：先看域名解析（dig/nslookup 确认返回的 IP 与 TTL 是否正确、是否被劫持或没生效），再测连通性（ping 看 ICMP 与丢包、curl 到 IP 看是否绑定证书问题、tracepath 看路径在哪跳断），然后看接入层（SLB/Nginx 的健康检查、后端 upstream 是否全挂、证书是否过期、WAF 是否拦截），最后进应用看日志、线程池与依赖。每一步都要用证据排除一层，并区分是影响所有用户还是特定地域/运营商，这直接决定是不是网络或 CDN 的问题。",
		Keywords:  []string{"dns", "dig", "ping", "curl", "tracepath", "负载均衡", "健康检查", "证书", "waf", "分层排查", "地域"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimProfessional,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "一次 K8s 滚动发布导致服务抖动，你会怎么复盘并改进发布流程？",
		Intent:    "考察变更管理与发布安全的实践深度。",
		Reference: "先还原事实：抖动的具体指标曲线、开始时间与发布批次的对应关系、受影响的请求比例。常见根因有新 Pod 未通过就绪探针就被摘入流量、maxUnavailable 过大导致可用副本不足、优雅停机时间短于在途请求、依赖的下游被打爆、镜像或配置变更。改进措施包括调整 maxSurge/maxUnavailable、加上 preStop 等待与连接排空、给启动慢的服务配 startup 探针、按批次放量并观察黄金指标、配置好自动回滚与发布窗口，并把这次故障写进发布检查清单和演练计划。",
		Keywords:  []string{"滚动发布", "maxsurge", "maxunavailable", "就绪探针", "优雅停机", "连接排空", "灰度放量", "自动回滚", "发布窗口", "复盘"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "你怎么做容量规划？大促前会准备哪些预案？",
		Intent:    "考察容量评估与应急准备的方法。",
		Reference: "容量规划从历史数据与业务目标反推：按峰值 QPS 与单机承载能力算出副本数，并留出安全水位（常见是压测到 70% 负载时仍能满足延迟目标）。用全链路压测验证瓶颈在网关、应用还是数据库，并找出单点与资源上限（连接数、文件句柄、带宽）。预案包括限流规则、降级开关（关非核心功能）、缓存预热、库存与配额预占、扩容与自动伸缩策略、值班与升级路径，并在此之前做演练确认开关真的可用。大促后要复盘实际峰值与预估的偏差，修正模型。",
		Keywords:  []string{"容量规划", "压测", "水位", "峰值qps", "扩容", "限流", "降级", "预案", "演练", "单点", "自动伸缩"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimAccuracy,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "IaC 和流水线你会怎么落地？如何保证生产变更可追溯、可回滚？",
		Intent:    "考察基础设施工程化的成熟度。",
		Reference: "核心原则是「基础设施即代码 + 不可变制品」：资源用 Terraform/云厂商模板声明，配置用 Helm/Kustomize 固化版本，所有变更走 Git 提交与评审，再由流水线执行 plan/apply，杜绝手工登录改生产。制品带版本与 commit 号，环境差异只通过变量区分。可追溯性靠审计日志、变更单与流水线记录三者关联；可回滚性要求每次变更都有对应回滚路径——镜像回切、配置切回、数据库变更向前兼容。还要配套最小权限与密钥管理，避免流水线成为最大的后门。",
		Keywords:  []string{"iac", "terraform", "helm", "流水线", "不可变制品", "代码评审", "审计日志", "回滚", "向前兼容", "最小权限", "密钥管理"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimProfessional,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "生产环境的权限怎么管？如何做到最小权限又不拖慢效率？",
		Intent:    "考察安全治理与协作成本的平衡。",
		Reference: "先做权限模型：按角色（RBAC）划分只读、运维、发布、管理四类，生产写权限默认不常驻，需要时通过申请审批临时授予并自动回收。所有登录生产必须走堡垒机并全程录屏审计，密钥与证书集中托管并支持轮换，禁止把凭证写进代码或 CI 变量明文。为了不拖慢效率，要把高频操作做成自助化的安全动作（如一键回滚、日志查询、扩缩容），让「走流程」比「绕流程」更省事。定期做权限复核，清理离职与闲置账号。",
		Keywords:  []string{"最小权限", "rbac", "堡垒机", "审计", "临时授权", "密钥管理", "轮换", "自助化", "权限复核", "合规"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "数据库被误删或者磁盘被打满，你的应急和恢复流程是什么？",
		Intent:    "考察数据安全与恢复能力的实操认知。",
		Reference: "先止损：磁盘满时先清理或扩容日志与临时文件、必要时只读挂起写入，避免数据继续损坏；误删时立刻停止写入并保护现场，防止覆盖可恢复的数据。恢复路径是「最近一次全量备份 + binlog 增量回放到故障点前」，所以必须先确认 RPO/RTO 目标并把备份重放演练做成例行事项——没演练过的备份等于没有备份。事后要补防线：删除走审批与延迟执行、高危语句审计、监控备份是否成功与磁盘水位，并给出明确的数据丢失范围说明。",
		Keywords:  []string{"备份", "binlog", "全量", "增量", "恢复演练", "rpo", "rto", "磁盘水位", "只读", "审计", "止损"},
	},
	{
		Family:    FamilyOps,
		Dimension: DimCommunication,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "你怎么向开发同事说明「这不是网络问题而是应用问题」？说说你的沟通方式。",
		Intent:    "考察跨团队协作中的证据意识与表达方式。",
		Reference: "关键是用证据替代结论：给出同一时刻的多组数据——网关侧的成功率与延迟、网络侧无丢包与重传、服务端的错误日志与线程堆积、以及从内网直连后端也能复现的结果，让判断自然成立。表达上先对齐共同目标（尽快恢复），再描述已经排除的层次和下一步需要谁配合，避免用「我们这边没问题」这种推责式口径。若确实无法立即定位，就约定分段验证的责任人和时间点，并在事后一起复盘，把协作方式沉淀下来。",
		Keywords:  []string{"证据", "排除法", "成功率", "日志", "责任边界", "协作", "复现", "共同目标", "复盘", "沟通"},
	},

	// --- test ------------------------------------------------------------
	{
		Family:    FamilyTest,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "一个登录功能你会怎么设计测试用例？说说你用到的用例设计方法。",
		Intent:    "考察测试用例设计的系统性与方法意识。",
		Reference: "先用等价类与边界值覆盖输入：手机号/邮箱格式、密码长度与字符集、验证码有效期、连续错误次数与账号锁定；再用场景法串起正常登录、记住我、多端登录、退出后 token 失效；异常路径包括网络中断、服务端 500、验证码过期、账号被禁用。还要考虑安全与兼容性：SQL 注入与暴力破解防护、密码是否明文传输、不同浏览器与机型。最后按优先级排序用例，明确哪些进回归集、哪些做自动化，并给出验收标准。",
		Keywords:  []string{"等价类", "边界值", "场景法", "用例设计", "异常路径", "验证码", "账号锁定", "优先级", "回归集", "安全"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "测试金字塔是什么？为什么单元测试性价比最高，而 UI 自动化往往最脆弱？",
		Intent:    "考察测试分层理论在工程中的真实效果。",
		Reference: "金字塔自下而上是单元、集成/接口、UI，数量依次减少：越靠下运行越快、越稳定、定位问题越准，成本越低。单元测试隔离了外部依赖，失败直接指向具体函数；UI 自动化要通过浏览器驱动与真实渲染，受等待、动画、选择器变化、环境差异影响，维护成本高且常出现 flaky，因此应只覆盖核心用户路径而不是替代下层测试。实践中许多团队是「冰淇淋筒」——UI 用例最多，结果是回归慢、信心低，需要通过契约测试和接口测试把重心下移。",
		Keywords:  []string{"测试金字塔", "单元测试", "集成测试", "接口测试", "ui自动化", "维护成本", "flaky", "反馈速度", "契约测试", "分层"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "线上出了一个漏测的严重缺陷，你会怎么复盘并防止再次发生？",
		Intent:    "考察复盘深度与流程改进意识。",
		Reference: "先还原事实链：缺陷的引入时间、被测试覆盖的情况、为什么没被用例捕捉到（场景缺失、数据特殊、环境差异、异步时序、还是需求理解偏差），以及为什么没被监控或灰度拦住。改进要落到机制上：补用例并入回归集、增加针对该场景的自动化或监控告警、调整发布策略（灰度观察时长、分批放量）、在需求评审阶段就把验收标准写清。复盘对事不对人，同时要评估同类风险还有哪些用例没覆盖，而不是只修好这一个 bug。",
		Keywords:  []string{"漏测", "根因", "复盘", "用例补充", "回归集", "灰度", "监控告警", "验收标准", "同类风险", "对事不对人"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimDepth,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "接口测试你会怎么设计？如何验证一个接口的幂等性和边界？",
		Intent:    "考察接口层测试的设计深度。",
		Reference: "先梳理接口契约：必填与可选参数、类型与范围、鉴权要求、响应结构与错误码。用例覆盖正常流程、参数边界（空值、超长、特殊字符、类型不符）、业务边界（库存为 1、金额为 0、重复提交）、以及异常路径（超时、下游报错、并发）。幂等性验证是同一请求（同一幂等键）重复发送 N 次且并发发送，断言业务结果只生效一次、返回一致；边界要靠等价类加边界值并做参数组合，避免穷举。自动化上做数据驱动与 schema 校验，接入 CI 每次构建都跑。",
		Keywords:  []string{"接口测试", "契约", "断言", "幂等", "并发", "重复提交", "边界值", "参数组合", "schema", "错误码", "数据驱动"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "性能测试怎么做？压测指标里你重点看哪些，如何定位瓶颈？",
		Intent:    "考察性能测试的完整闭环能力。",
		Reference: "先定目标：核心接口在目标并发下的 RT 与错误率要求，并准备接近生产的测试数据与独立环境。压测用梯度加压找拐点，关注 TPS、平均与 p95/p99 延迟、错误率、以及系统资源曲线；只看平均值会被长尾掩盖。定位瓶颈按层次看：应用侧的 GC 日志与线程/协程阻塞、连接池与线程池排队、慢 SQL 与锁等待、缓存命中率，以及网络与外部依赖。最后要验证扩容与限流阈值是否有效，并把结论写成容量结论（单副本承载多少、需要多少副本）。",
		Keywords:  []string{"压测", "tps", "p99", "梯度加压", "拐点", "错误率", "瓶颈", "gc", "连接池", "慢sql", "容量结论"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimAccuracy,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "自动化测试框架如何选型？什么情况下你会建议团队不要做 UI 自动化？",
		Intent:    "考察技术选型判断而不只是工具偏好。",
		Reference: "选型看四点：团队技术栈与学习成本、用例可维护性（选择器策略、页面对象或关键字驱动）、报告与 CI 集成能力、社区活跃度与生态（如 Playwright 的内置等待与录制、Selenium 的广泛兼容、pytest 的插件生态）。是否做 UI 自动化取决于收益比：需求频繁大改、页面结构不稳定、用例一年跑不了几次、或同样的验证能用接口测试覆盖时，就不该投 UI 自动化，改成少量核心路径的冒烟用例加人工探索测试更划算。",
		Keywords:  []string{"自动化", "选型", "playwright", "selenium", "pytest", "可维护性", "选择器", "ci集成", "冒烟用例", "收益", "探索测试"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimProfessional,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "测试左移和持续集成门禁你怎么落地？怎么让开发愿意写测试？",
		Intent:    "考察质量体系建设的推动能力。",
		Reference: "左移不是喊口号，而是把活动前置到需求与设计阶段：评审时确认验收标准与边界，开发阶段提供可测性（依赖注入、可控时钟、mock 外部依赖），提测前用静态检查与单测拦住低级问题。门禁要务实：先卡 lint 与冒烟用例、再逐步提高覆盖率阈值，把新增代码覆盖率作为门槛而不是全量指标，避免为了数字写无效测试。推动开发意愿靠降低摩擦——提供测试脚手架与夹具、让失败反馈足够快且清晰、把测试失败的修复成本可视化，并把质量结果纳入团队而非个人的评价。",
		Keywords:  []string{"测试左移", "门禁", "ci", "增量覆盖率", "验收标准", "可测性", "mock", "脚手架", "反馈速度", "质量文化"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "异步任务、定时任务和第三方依赖这类难测场景，你怎么保证覆盖？",
		Intent:    "考察对不可控依赖的测试设计能力。",
		Reference: "原则是把不确定的东西变成可注入的确定接口：时间通过时钟抽象注入，让测试能跳到「昨天 23:59」这类边界；异步与消息队列用可控的消费者驱动或等待断言（轮询直到超时），避免 sleep；第三方依赖在集成测试里用契约测试或录制回放（如 WireMock、sandbox 环境），在单元测试里用桩，并单独覆盖超时、重试、幂等和错误码分支。定时任务要测重复触发与并发触发，并把补偿逻辑（漏跑任务如何补）纳入用例。",
		Keywords:  []string{"异步", "定时任务", "时钟注入", "mock", "契约测试", "录制回放", "超时", "重试", "幂等", "边界", "补偿"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimAccuracy,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "代码覆盖率 90% 就一定质量好吗？请说说覆盖率的局限。",
		Intent:    "考察对度量指标的批判性理解。",
		Reference: "覆盖率只说明代码被执行过，不说明结果被验证过：没有断言的测试、断言过弱的测试都能把覆盖率刷上去，而真正的边界与异常分支可能在 10% 的未覆盖代码里。行覆盖比分支覆盖宽松，条件组合更难覆盖。合理用法是把它当作发现「完全没测到」区域的探照灯，而不是质量目标，尤其要盯住核心业务分支和缺陷历史上集中的模块（缺陷密度比覆盖率更能指引测试投入）。",
		Keywords:  []string{"覆盖率", "行覆盖", "分支覆盖", "断言", "无效测试", "边界", "缺陷密度", "度量", "质量目标", "探照灯"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimCommunication,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "开发说「这是需求问题不是 bug」，你怎么处理这类分歧？",
		Intent:    "考察以事实为依据的协作能力。",
		Reference: "先回到可验证的事实：拿出复现步骤、影响面、与需求文档或原型的对照，明确当前行为与预期行为的差异到底在哪一行文字上。若确实是需求表述有歧义，就把它升级为需求澄清，让产品给出结论并更新文档，而不是在群里争输赢。处理过程中要记录结论与责任人，避免同一问题反复出现。若影响线上用户，先按「当前最合理的预期」提交缺陷并同步产品，止损优先于定责。",
		Keywords:  []string{"复现步骤", "需求文档", "预期行为", "影响面", "需求澄清", "验收标准", "记录结论", "止损", "对齐", "协作"},
	},
	{
		Family:    FamilyTest,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "如果让你从零搭一个测试平台，你会先解决团队的哪个痛点？",
		Intent:    "考察工具建设的价值判断能力。",
		Reference: "先做调研而不是先写代码：看团队当前的瓶颈是用例散落无版本管理、执行靠手工、结果无法追溯，还是环境数据准备成本高。多数团队的第一个痛点是「跑起来和看得懂」，所以第一步通常是用例与套件管理 + 一键触发 + 报告聚合，接入 CI 让结果自动回流。第二步再解决效率问题：并行调度、环境隔离、数据构造与清理、失败自动重跑并标注 flaky。建设要有度量（回归耗时、缺陷逃逸率、用例通过率）证明价值，避免做成没人用的面子工程。",
		Keywords:  []string{"测试平台", "痛点调研", "用例管理", "一键触发", "报告聚合", "调度", "环境隔离", "数据构造", "flaky", "度量", "roi"},
	},

	// --- generic ---------------------------------------------------------
	{
		Family:    FamilyGeneric,
		Dimension: DimCommunication,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "请用三分钟讲一个你最有成就感的项目，说清背景、你的角色和最终结果。",
		Intent:    "考察结构化表达与结果意识。",
		Reference: "按 STAR 讲：先说清业务背景与要解决的问题，再明确你个人的职责边界（哪部分是你做的、团队怎么分工），然后讲关键决策与遇到的难点以及你怎么取舍，最后用数据或事实给结果，并补一句影响面（被多少用户使用、节省多少人力、是否被复用）。常见扣分点是只讲团队成果不讲个人贡献、只讲做了什么不讲为什么这么做、结果没有量化。",
		Keywords:  []string{"背景", "目标", "角色", "行动", "结果", "量化", "难点", "取舍", "star", "复盘"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimTeamwork,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "讲一次你和同事意见不合的经历，最后是怎么解决的？",
		Intent:    "考察协作方式与冲突处理成熟度。",
		Reference: "好的回答会先还原分歧的实质（是对目标理解不同、还是对方案代价判断不同），再说明你做了什么：先听完对方的依据、把争议点收敛到可验证的事实或数据上、必要时做小范围试验或原型取信，最后达成共识或由负责人拍板，并且执行时不打折扣。要避免两种极端：一味妥协不表达观点，或坚持己见靠情绪施压。结尾可以说明事后关系与合作方式的变化。",
		Keywords:  []string{"分歧", "倾听", "依据", "数据", "共识", "小范围试验", "拍板", "执行", "对事不对人", "协作"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimSelfAwareness,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "你觉得自己最大的短板是什么？为此做过什么具体努力？",
		Intent:    "考察自我认知的诚实度与改进行动力。",
		Reference: "回答要选真实但不致命的短板，例如技术广度不足、公开表达紧张、跨部门推动时过于被动，而不是「我太追求完美」这类回避式答案。重点在改进动作：你用了什么方法（刻意练习、找反馈、承担对应任务）、持续了多久、现在到了什么程度、还有什么没解决。能说出一个可验证的变化（例如从不敢讲变成主持过几次技术分享）比表态更有说服力。",
		Keywords:  []string{"短板", "真实", "改进动作", "反馈", "刻意练习", "持续", "验证", "自省", "成长", "未解决"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimFit,
		Levels:    []string{LevelJunior, LevelMid},
		Question:  "为什么选择我们公司这个岗位？你未来三年的规划是什么？",
		Intent:    "考察求职动机与岗位匹配度。",
		Reference: "回答要能对上具体信息：公司的业务方向与岗位职责你为什么感兴趣、你的经历与岗位要求哪里契合、你希望在这里补齐什么能力，而不是泛泛夸公司。三年规划要具体而不空泛：例如一年内独立负责某类模块、两年内具备方案设计与带人能力，并说明这与岗位的成长路径一致。同时要诚实说明你的优先级（技术深度、业务复杂度、团队氛围），让面试官判断稳定性与预期是否匹配。",
		Keywords:  []string{"岗位匹配", "业务方向", "职责", "契合", "成长路径", "规划", "动机", "稳定性", "预期", "学习"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimSituational,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "项目上线前一天发现一个可能影响核心流程的问题，你会怎么决策？",
		Intent:    "考察风险判断与压力下的取舍。",
		Reference: "先快速量化风险：影响哪些用户、触发概率多大、有没有绕过或绕开的手段、最坏后果是什么。然后给决策者几个明确选项而不是抛问题——按原计划上线并加监控与开关、缩小灰度范围先放小流量、或者延期修复。同时确认回滚路径是否真的可用（回滚会不会丢数据、配置能否切回），并把判断依据和结论同步到相关方。要点是既不隐瞒风险硬上，也不因小概率问题一刀切延期。",
		Keywords:  []string{"风险评估", "影响面", "概率", "灰度", "开关", "回滚方案", "延期", "同步", "决策选项", "止损"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimProblemSolving,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "接手一个没有文档、原作者已离职的老系统，你会怎么快速上手？",
		Intent:    "考察学习路径与主动性。",
		Reference: "先建立全局地图再钻细节：从入口路由、部署结构与数据库表关系画出调用链路，跑一遍核心业务并记录每一步的数据变化。然后借助现成信号——监控指标、日志、告警历史、最近的提交记录和需求变更，判断哪些模块最活跃、最容易出问题。同时主动向使用方（客服、运营、下游团队）问问题，他们往往比代码更快说清边界。过程中自己补一份文档与踩坑笔记，用小改动验证理解，避免一上来就重构。",
		Keywords:  []string{"调用链路", "数据库表关系", "监控", "日志", "提交记录", "访谈", "文档", "小改动验证", "技术债", "笔记"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimLogic,
		Levels:    []string{LevelMid, LevelSenior},
		Question:  "讲一个你用数据或实验推动决策的例子，你怎么证明结论是可靠的？",
		Intent:    "考察论证链条的完整性与因果意识。",
		Reference: "按「问题—假设—验证—结论—行动」讲：先说清原来的分歧与需要做的决定，把你的判断写成可证伪的假设；再说验证方式（对照实验、分群对比、灰度放量），讲明如何排除其他解释（时间趋势、季节性、样本偏差、同时发生的其他变更）；给出数据结果与置信程度，并说明你据此做了什么决定、结果如何。如果无法做实验，也要说明用什么证据链支撑，以及你如何控制风险。",
		Keywords:  []string{"假设", "可证伪", "对照实验", "分群", "灰度", "样本偏差", "趋势", "置信", "结论", "行动", "因果"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimSelfAwareness,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "回顾你的职业生涯，哪次选择你认为最关键？当时的取舍是什么？",
		Intent:    "考察长期视角与决策框架的成熟度。",
		Reference: "好回答会讲清当时的约束条件（行业趋势、个人能力短板、家庭与城市、团队机会），说明你比较过哪些选项、用什么标准排序（成长速度、技术积累、业务前景还是收入），以及你主动放弃了什么。重点是体现判断依据而非事后复盘式的「运气好」：能承认当时信息不足、后来发现判断有偏差，并说明这次选择如何影响了你后来的路径。这类问题看的是稳定性和自我认知，不是故事讲得多精彩。",
		Keywords:  []string{"关键选择", "约束条件", "取舍", "标准", "长期", "成长", "放弃", "判断依据", "偏差", "路径"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimTeamwork,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "你如何推动一个跨团队、没有直接汇报关系的项目落地？",
		Intent:    "考察非职权影响力与项目推进能力。",
		Reference: "核心是让每个参与方都能看到自己的收益，而不只是帮你完成任务：先与关键方一对一沟通摸清诉求与顾虑，把目标翻译成对各方都成立的语言，再定出共同的成功标准与里程碑。机制上要有明确的负责人、周节奏与风险清单，问题和变更走书面同步以免信息失真。遇到阻塞时向上借力但不越级甩锅，用数据说明不做的代价。收尾时公开认领功劳、复盘流程，为下一次协作积累信任。",
		Keywords:  []string{"共同目标", "收益", "一对一", "成功标准", "里程碑", "责任人", "风险清单", "书面同步", "向上借力", "复盘"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimCommunication,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "如果让你给团队做一次技术分享，你会怎么选题和组织内容？",
		Intent:    "考察知识传播能力与受众意识。",
		Reference: "先定受众与目标：听众是该领域的初学者还是有经验的同行，你希望他们听完能做什么（能上手用、能避开某个坑、能参与讨论），这决定深度与时长。选题优先选自己踩过坑、有真实数据或代码的题目，讲「我们遇到的问题和取舍」比讲 API 更有价值。结构上从一个具体场景切入，讲清方案、代价与失败经验，中间穿插可运行的示例或现场排障，最后给出可落地的清单与延伸阅读。结束后收集反馈并沉淀成文档。",
		Keywords:  []string{"受众", "目标", "选题", "实际案例", "坑", "结构", "示例", "取舍", "延伸阅读", "反馈", "沉淀"},
	},
	{
		Family:    FamilyGeneric,
		Dimension: DimSituational,
		Levels:    []string{LevelSenior, LevelExpert},
		Question:  "你最失败的一次项目经历是什么？如果重来你会怎么做？",
		Intent:    "考察抗压复盘与成长的真实性。",
		Reference: "回答要敢于讲真正的失败（项目延期、方向做错、团队流失），并说清你在其中的具体责任而不是全推给外部。重点有三段：失败的事实与后果、你事后找到的根因（假设没验证、范围失控、风险没暴露、沟通不足）、以及你现在具体改变了什么做法。重来的部分要给出可执行的方案（先做小范围验证、设里程碑与止损点、提前对齐关键方），并能举出事后另一次项目中这些改进确实生效的证据。",
		Keywords:  []string{"失败", "责任", "后果", "根因", "假设验证", "范围失控", "风险暴露", "止损点", "改进", "证据", "复盘"},
	},
}

// roleFamilyRule is one ordered classifier rule: the first family whose
// needles appear anywhere in the lowercased role title wins.
//
// WHY a flat ordered list instead of a map: order is the whole algorithm
// here. "测试开发工程师" contains 开发 and must land in test, "全栈后端"
// must land in fullstack before backend, and "数据产品经理" is a product
// role rather than a data one. Encoding that as list order keeps the
// precedence visible and reviewable.
type roleFamilyRule struct {
	family  string
	needles []string
}

// roleFamilyRules is checked top to bottom; more specific families come
// first. Every needle is lowercase because the input is lowercased.
var roleFamilyRules = []roleFamilyRule{
	{FamilyTest, []string{
		"测试", "测开", "qa", "sdet", "质量", "testing", "test engineer", "自动化测试", "用例",
	}},
	{FamilyOps, []string{
		"运维", "sre", "devops", "k8s", "kubernetes", "云原生", "基础设施", "平台工程",
		"系统管理", "dba", "site reliability", "安全工程师", "监控",
	}},
	{FamilyProduct, []string{
		"产品", "product", "pm", "运营", "交互设计", "用户体验", "ue", "ui设计",
	}},
	{FamilyAlgorithm, []string{
		"算法", "机器学习", "深度学习", "人工智能", "大模型", "llm", "nlp", "计算机视觉",
		"数据挖掘", "推荐", "搜索算法", "风控建模", "ai工程师", "cv工程师",
	}},
	{FamilyData, []string{
		"数据", "数据分析", "数仓", "大数据", "etl", "bi", "data", "hive", "spark", "flink",
		"统计", "报表",
	}},
	{FamilyFullstack, []string{
		"全栈", "fullstack", "full-stack", "full stack", "全端",
	}},
	{FamilyBackend, []string{
		"后端", "backend", "back-end", "back end", "服务端", "server", "java", "golang",
		"spring", "php", "python", "c++", "c#", ".net", "微服务", "中间件", "架构师", "分布式",
	}},
	{FamilyFrontend, []string{
		"前端", "frontend", "front-end", "front end", "react", "vue", "angular", "h5",
		"小程序", "web", "页面", "javascript", "typescript", "css", "客户端",
	}},
}

// BankQuestions returns the whole catalogue.
//
// The returned slice is a deep copy: callers (plan building, tests, the
// API) may filter or mutate what they get, and a shallow copy would let
// them corrupt the shared backing array of Levels/Keywords for every
// later call. Deep-copying two short string slices per question is cheap
// next to that risk.
func BankQuestions() []BankQuestion {
	out := make([]BankQuestion, len(bankCatalogue))
	for i, q := range bankCatalogue {
		q.Levels = append([]string(nil), q.Levels...)
		q.Keywords = append([]string(nil), q.Keywords...)
		out[i] = q
	}
	return out
}

// RoleFamily classifies a free-text job title (Chinese or English) into a
// Family* slug, falling back to FamilyGeneric.
//
// Matching is case-insensitive substring matching, ordered by
// specificity, so a title that mentions several domains resolves to the
// more specialised family (see roleFamilyRules).
func RoleFamily(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	if r == "" {
		return FamilyGeneric
	}
	for _, rule := range roleFamilyRules {
		for _, n := range rule.needles {
			if strings.Contains(r, n) {
				return rule.family
			}
		}
	}
	return FamilyGeneric
}

// presets is the built-in one-click setup list for the wizard. Nine
// roles, chosen to cover each family at a plausible level so a first-time
// user can start an interview in one click without writing a JD.
var presets = []Preset{
	{
		ID:            "backend-senior",
		Role:          "高级后端工程师",
		Level:         LevelSenior,
		InterviewType: TypeTech,
		Difficulty:    DifficultyHard,
		QuestionCount: 10,
		FocusAreas:    []string{"Go/Java 并发", "MySQL 调优", "缓存与消息队列", "分布式一致性", "高并发设计"},
		JDSample:      "我们正在招聘高级后端工程师，负责核心交易链路的架构设计与性能优化。要求扎实的计算机基础，熟悉 Go 或 Java 中至少一门语言的运行时与并发模型，具备 MySQL 索引与事务调优、Redis 缓存治理、消息队列削峰填谷的实战经验。有分库分表、分布式事务、高并发秒杀或支付类系统经验者优先。你将主导技术方案评审，并推动线上稳定性与可观测性建设。",
		Description:   "面向有 3-5 年经验的后端候选人，重点考察并发、存储与分布式一致性。",
	},
	{
		ID:            "frontend-mid",
		Role:          "前端工程师",
		Level:         LevelMid,
		InterviewType: TypeTech,
		Difficulty:    DifficultyNormal,
		QuestionCount: 8,
		FocusAreas:    []string{"React 原理", "CSS 与布局", "性能优化", "浏览器与网络", "前端工程化"},
		JDSample:      "招聘前端工程师，负责公司 SaaS 产品的中后台与用户端页面开发。技术栈为 React 18 + TypeScript，构建工具使用 Vite，要求熟悉组件化开发、状态管理与前端路由，理解浏览器渲染、事件循环与缓存机制。能够独立完成性能优化与首屏指标改善，并与设计、后端紧密配合。有 SSR、微前端或组件库建设经验者优先。",
		Description:   "面向 2-4 年前端，围绕 React 原理、布局与性能优化展开。",
	},
	{
		ID:            "fullstack-mid",
		Role:          "全栈工程师",
		Level:         LevelMid,
		InterviewType: TypeMixed,
		Difficulty:    DifficultyNormal,
		QuestionCount: 9,
		FocusAreas:    []string{"接口设计", "Go 后端", "React 前端", "部署与运维", "安全"},
		JDSample:      "我们是一个小团队，招聘全栈工程师独立负责一条产品线从接口到页面的完整交付。后端为 Go + SQLite/PostgreSQL，前端为 React + TypeScript，需要你具备接口契约设计、鉴权与权限、ORM 性能与部署上线的完整能力。你还会参与需求评审、排期与技术选型。希望你有独立负责过一个完整项目的经历，并在遇到问题时能自己排查到根因。",
		Description:   "适合小团队场景，考察从接口到部署的端到端交付能力。",
	},
	{
		ID:            "data-analyst-mid",
		Role:          "数据分析师",
		Level:         LevelMid,
		InterviewType: TypeMixed,
		Difficulty:    DifficultyNormal,
		QuestionCount: 8,
		FocusAreas:    []string{"SQL 与数仓", "指标体系", "AB 实验", "漏斗与留存", "数据质量"},
		JDSample:      "招聘数据分析师，负责业务核心指标体系的搭建与异动归因，为产品与运营决策提供依据。要求熟练使用 SQL 与至少一种数仓工具，理解分层建模与口径管理，能从海量数据中定位问题并给出可执行建议。需要具备实验设计与显著性判断能力，能独立推动一次 AB 实验从设计到结论。有埋点治理或数据质量监控经验者优先。",
		Description:   "面向业务分析岗，重点考察 SQL、指标体系与实验设计。",
	},
	{
		ID:            "algo-senior",
		Role:          "算法工程师",
		Level:         LevelSenior,
		InterviewType: TypeTech,
		Difficulty:    DifficultyHard,
		QuestionCount: 9,
		FocusAreas:    []string{"机器学习基础", "特征工程", "深度学习原理", "推荐/搜索", "模型上线"},
		JDSample:      "招聘算法工程师，负责推荐与搜索排序模型的迭代优化。要求扎实的机器学习与深度学习基础，熟悉特征工程、模型评估与调参，能独立完成从数据处理、模型训练到线上服务的闭环。需要了解召回、粗排、精排的链路设计与线上延迟约束。有 Transformer、大模型微调或特征平台经验者优先。",
		Description:   "面向推荐/搜索方向，考察机器学习原理与线上线下一致性。",
	},
	{
		ID:            "product-senior",
		Role:          "产品经理",
		Level:         LevelSenior,
		InterviewType: TypeBehavior,
		Difficulty:    DifficultyNormal,
		QuestionCount: 8,
		FocusAreas:    []string{"需求分析", "数据驱动决策", "跨部门协作", "优先级管理", "商业化"},
		JDSample:      "招聘产品经理，负责企业服务产品的需求规划与落地。你需要深入业务场景挖掘真实痛点，输出清晰的需求文档与原型，并推动设计、研发、测试、运营协同交付。要求具备数据敏感度，能为每次迭代定义可衡量的目标并做上线复盘。有跨部门推动复杂项目、或商业化付费功能设计经验者优先。",
		Description:   "行为面试为主，考察需求判断、数据决策与跨部门推进能力。",
	},
	{
		ID:            "sre-senior",
		Role:          "运维/SRE 工程师",
		Level:         LevelSenior,
		InterviewType: TypeTech,
		Difficulty:    DifficultyHard,
		QuestionCount: 9,
		FocusAreas:    []string{"K8s 与容器", "监控告警", "故障排查", "容量与预案", "变更与安全"},
		JDSample:      "招聘 SRE 工程师，负责生产环境稳定性与基础设施工程化。要求熟悉 Kubernetes 的调度、探针与发布策略，能基于 Prometheus/Grafana 建设指标体系与告警，并在故障中快速定位与止血。需要具备容量规划、压测与预案演练经验，推动 IaC 与流水线落地。有大规模集群治理、成本优化或安全加固经验者优先。",
		Description:   "面向稳定性方向，考察 K8s、可观测性与故障应急能力。",
	},
	{
		ID:            "testdev-mid",
		Role:          "测试开发工程师",
		Level:         LevelMid,
		InterviewType: TypeTech,
		Difficulty:    DifficultyNormal,
		QuestionCount: 8,
		FocusAreas:    []string{"测试策略", "接口自动化", "性能测试", "CI 门禁", "质量度量"},
		JDSample:      "招聘测试开发工程师，负责核心业务的测试策略设计与自动化体系搭建。要求掌握等价类、边界值等用例设计方法，能独立设计接口与 UI 自动化方案并接入 CI 流水线。需要具备性能压测与瓶颈定位能力，能从漏测事故中复盘并推动流程改进。有测试平台或质量度量体系建设经验者优先。",
		Description:   "面向测试开发，考察测试设计、自动化落地与质量体系推动。",
	},
	{
		ID:            "campus-junior",
		Role:          "校招初级工程师",
		Level:         LevelJunior,
		InterviewType: TypeMixed,
		Difficulty:    DifficultyEasy,
		QuestionCount: 8,
		FocusAreas:    []string{"计算机基础", "编码能力", "项目经历", "学习能力", "沟通表达"},
		JDSample:      "校园招聘初级工程师，欢迎计算机相关专业的应届毕业生投递。我们希望你有扎实的编程基础与数据结构、操作系统、网络知识，至少完成过一个完整的课程或实习项目并能讲清自己的贡献。我们更看重学习能力、解决问题的思路与团队协作意识，入职后会配备导师并参与真实业务开发。",
		Description:   "校招/初级岗，兼顾基础技术题与行为面试，难度较低。",
	},
	{
		ID:            "architect-expert",
		Role:          "资深架构师",
		Level:         LevelExpert,
		InterviewType: TypeSystemDesign,
		Difficulty:    DifficultyHard,
		QuestionCount: 10,
		FocusAreas:    []string{"系统设计", "高可用与一致性", "容量与成本", "技术治理", "架构演进"},
		JDSample:      "招聘资深架构师，负责公司级技术架构规划与关键系统的设计评审。要求具备大型分布式系统的设计与落地经验，能在一致性、可用性、成本之间做出可解释的取舍，并推动架构逐步演进而不是一次性重写。你需要主导容量规划、稳定性治理与技术债偿还，同时具备跨团队的技术影响力与方案表达能力。",
		Description:   "系统设计面试，考察架构取舍、容量估算与技术治理判断。",
	},
}

// Presets returns the built-in role presets.
//
// Like BankQuestions this is a deep copy, because handlers hand these to
// the frontend and the wizard may normalize fields in place; the shared
// backing arrays (FocusAreas especially) must not leak.
func Presets() []Preset {
	out := make([]Preset, len(presets))
	for i, p := range presets {
		p.FocusAreas = append([]string(nil), p.FocusAreas...)
		out[i] = p
	}
	return out
}
