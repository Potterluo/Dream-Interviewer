package com.yupi.springbootinit.controller;

import com.yupi.springbootinit.model.dto.moonshotai.PostSendMessages;
import com.yupi.springbootinit.model.dto.moonshotai.PostSendRequest;

import org.springframework.web.bind.annotation.*;
import org.springframework.http.*;
import org.springframework.web.client.RestTemplate;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/chat")
public class ChatController {

    private static final String API_BASE_URL = "https://api.moonshot.cn/v1";
    private static final String API_KEY = ""; // 请替换为你的实际API密钥

    @GetMapping("/test")
    public ResponseEntity<String> chat(@RequestParam String userMessage) {
        // 创建请求体
        Map<String, Object> requestBody = new HashMap<>();
        requestBody.put("model", "moonshot-v1-8k");
        /*requestBody.put("messages", List.of(
                new HashMap<String, String>() {{
                    put("role", "system");
                    put("content", "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。");
                }},
                new HashMap<String, String>() {{
                    put("role", "user");
                    put("content", userMessage);
                }}
        ));*/
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

    @GetMapping("/test2")
    public ResponseEntity<String> chat2(PostSendRequest sendMessage) {
        PostSendRequest realMessage = new PostSendRequest();
        realMessage.setModel("moonshot-v1-8k");
        realMessage.setTemperature(0.3);
        PostSendMessages message1  = new PostSendMessages();
        PostSendMessages message2  = new PostSendMessages();
        PostSendMessages message3  = new PostSendMessages();
        PostSendMessages message4  = new PostSendMessages();
        message1.setRole("system");
        message1.setContent("你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。");
        message2.setRole("user");
        message2.setContent("你好，我叫李雷，1+1等于多少？");
        message3.setRole("assistant");
        message3.setContent("你好，李雷！1+1等于2");
        message4.setRole("user");
        message4.setContent("你好，我的上一个问题是什么？");
        //List<PostSendMessages> messages = List.of(message1, message2, message3, message4);//Java 9的新特性
        List<PostSendMessages> messages = new ArrayList<>();
        messages.add(message1);
        messages.add(message2);
        messages.add(message3);
        messages.add(message4);
        realMessage.setMessages(messages);

        RestTemplate restTemplate = new RestTemplate();
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        headers.set("Authorization", "Bearer " + API_KEY);
        HttpEntity<Object> entity = new HttpEntity<>(realMessage, headers);
        System.out.println(realMessage.toString());
        System.out.println(realMessage.getModel());
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
        //
    }
}