package com.yupi.springbootinit.controller;

import org.elasticsearch.core.List;
import org.springframework.web.bind.annotation.*;
import org.springframework.http.*;
import org.springframework.web.client.RestTemplate;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.util.HashMap;
import java.util.Map;

@RestController
@RequestMapping("/")
public class ChatController {

    private static final String API_BASE_URL = "https://api.moonshot.cn/v1";
    private static final String API_KEY = "sk-2AMgj1sLHrhD6ymq7dR4vPInCHzJREpHcd2XY3voZhJ2Yaax"; // 请替换为你的实际API密钥

    @GetMapping("/chat")
    public ResponseEntity<String> chat(@RequestParam String userMessage) {
        // 创建请求体
        Map<String, Object> requestBody = new HashMap<>();
        requestBody.put("model", "moonshot-v1-8k");
        requestBody.put("messages", List.of(
                new HashMap<String, String>() {{
                    put("role", "system");
                    put("content", "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。");
                }},
                new HashMap<String, String>() {{
                    put("role", "user");
                    put("content", userMessage);
                }}
        ));
        requestBody.put("temperature", 0.3);

        // 使用RestTemplate发送请求
        RestTemplate restTemplate = new RestTemplate();
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        headers.set("Authorization", "Bearer " + API_KEY);
        HttpEntity<Object> entity = new HttpEntity<>(requestBody, headers);

        try {
            // 发送POST请求到Moonshot AI的API
            ResponseEntity<String> response = restTemplate.exchange(
                    API_BASE_URL + "/chat/completions",
                    HttpMethod.POST,
                    entity,
                    String.class);
            // 返回API响应
            return response;
        } catch (Exception e) {
            // 如果发生错误，返回错误信息
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                    .body("Error: " + e.getMessage());
        }
    }
}