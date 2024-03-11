package com.yupi.springbootinit.service.impl;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.yupi.springbootinit.model.dto.context.request.ContextRequest;
import com.yupi.springbootinit.model.dto.context.request.GlobalMessageList;
import com.yupi.springbootinit.model.dto.context.response.FileContentResponse;
import com.yupi.springbootinit.model.dto.context.response.UploadFileResopnse;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.core.io.FileSystemResource;
import org.springframework.http.*;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.client.HttpClientErrorException;
import org.springframework.web.client.HttpServerErrorException;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.multipart.MultipartFile;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.Random;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;

import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpSession;

@Service
public class CvFileService {
    // @Value("${moonshot.api.key}")
    static String MOONSHOT_API_KEY = "sk-EuMYqAL9j2h3aa1Pk8jqN0b49bdqJGp1JZLR1Cpn2faICAdh";
    //@Value("${moonshot.api.base_url}")
    static String MOONSHOT_API_BASE_URL = "https://api.moonshot.cn/v1";

    private final RestTemplate restTemplate;

    /*private final GlobalMessageList globalMessageList;*/

/*
    @Autowired
    public CvFileService(GlobalMessageList globalMessageList){
        this.globalMessageList = globalMessageList;
    }
*/


    public CvFileService(RestTemplateBuilder restTemplateBuilder) {
        this.restTemplate = restTemplateBuilder
                .setConnectTimeout(Duration.ofSeconds(30))
                .setReadTimeout(Duration.ofSeconds(30))
                .build();
    }

    public String getFileContent(String fileId) {
        // 构建请求URL
        String url = MOONSHOT_API_BASE_URL+"/files/" + fileId + "/content";
        // 设置请求头
        HttpHeaders headers = new HttpHeaders();
        headers.set("Authorization", "Bearer " + MOONSHOT_API_KEY);
        headers.setContentType(MediaType.APPLICATION_JSON);

        try {
            // 创建请求实体
            HttpEntity<String> request = new HttpEntity<>("", headers);

            // 发送GET请求
            ResponseEntity<String> response = restTemplate.exchange(
                    url,
                    HttpMethod.GET,
                    request,
                    String.class);
            // 检查响应状态并返回文件内容
            if (response.getStatusCode() == HttpStatus.OK) {
                System.out.println(response.getBody());
                FileContentResponse fileContentResponse = new ObjectMapper().readValue(response.getBody(), FileContentResponse.class);
                return fileContentResponse.getContent();
            }else {
                System.out.println("Failed to get file content due to HTTP error.");
                System.out.println("Error message: " + response.getBody());
                return null;
            }
        }catch (HttpClientErrorException | HttpServerErrorException ex) {
            System.out.println("Failed to upload file due to HTTP error.");
            System.out.println("Error message: " + ex.getMessage());
            ex.printStackTrace();
            return null;
        } catch (Exception ex) {
            System.out.println("Failed to upload file due to unexpected exception.");
            System.out.println("Error message: " + ex.getMessage());
            ex.printStackTrace();
            return null;
        }

    }



    //TODO 自定义类获取文件ID
    public static String uploadMSAiFiles(File file) throws JsonProcessingException {
        // 创建RestTemplate实例
        RestTemplate restTemplate = new RestTemplate();

        // 设置请求头
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.MULTIPART_FORM_DATA);
        headers.set("Authorization", "Bearer " + MOONSHOT_API_KEY);

        // 创建多部分请求体
        MultiValueMap<String, Object> body = new LinkedMultiValueMap<>();
        body.add("file", new FileSystemResource(file)); // 将MultipartFile转换为FileSystemResource添加到请求体
        body.add("purpose", "file-extract");

        // 创建请求实体
        HttpEntity<MultiValueMap<String, Object>> request = new HttpEntity<>(body, headers);

        // 发送POST请求
        String MOONSHOT_FILE_API_BASE_URL = MOONSHOT_API_BASE_URL + "/files";
        try {
            ResponseEntity<String> response = restTemplate.postForEntity(MOONSHOT_FILE_API_BASE_URL, request, String.class);
            System.out.println("Response status: " + response.getStatusCode());
            System.out.println("Response body: " + response.getBody());
            UploadFileResopnse uploadFileResopnse = new ObjectMapper().readValue(response.getBody(), UploadFileResopnse.class);

            return uploadFileResopnse.getId();

        } catch (HttpClientErrorException | HttpServerErrorException ex) {
            System.out.println("Failed to upload file due to HTTP error.");
            System.out.println("Error message: " + ex.getMessage());
            ex.printStackTrace();
            return null;
        } catch (Exception ex) {
            System.out.println("Failed to upload file due to unexpected exception.");
            System.out.println("Error message: " + ex.getMessage());
            ex.printStackTrace();
            return null;
        }
    }
    //@Value("${file.upload.path}")
    private String uploadPath = "E:\\TestUpload";

    public ResponseEntity<String> uploadfile(@RequestParam("cvfile")  MultipartFile file , HttpServletRequest request) {
        if (file.isEmpty()) {
            return new ResponseEntity<>("请选择文件", HttpStatus.BAD_REQUEST);
        }

        try {
            String fileName = String.format("%06d", new Random().nextInt(900000) + 1) + file.getOriginalFilename();
            // 设置文件存储路径
            Path path = Paths.get(uploadPath, fileName);
            // 保存文件
            Files.copy(file.getInputStream(), path);
            File upfile = new File("E:\\TestUpload\\" + fileName);
            CvFileService cvFileService = new CvFileService(new RestTemplateBuilder());
            // 上传文件至Moonshot
            String fileId = cvFileService.uploadMSAiFiles(upfile);
            //抽取文件内容
            String fileContent = cvFileService.getFileContent(fileId);
            if(null != fileContent){
/*                ContextRequest contextRequest = new ContextRequest();
                System.out.println("开始注入session");
                HttpSession session = request.getSession();
                System.out.println("存在session"+session.getAttribute("sessionMessageRequest"));
                //TODO 前端传入信息
                contextRequest.setMessageRole("user");
                contextRequest.setMessageContent(fileContent);
                contextRequest.setListId(124124);
                contextRequest.setUserAccount("keriko");
                SessionMessageService sessionMessageService = new SessionMessageService();
                sessionMessageService.addMessageList(request,contextRequest);*/
                //文件加入session
                return new ResponseEntity<>(file.getOriginalFilename() + ":\n" + fileContent, HttpStatus.OK);
            }else {
                return new ResponseEntity<>("文件上传失败", HttpStatus.INTERNAL_SERVER_ERROR);
            }

        } catch (Exception e) {
            e.printStackTrace();
            return new ResponseEntity<>("文件上传失败", HttpStatus.INTERNAL_SERVER_ERROR);
        }

    }
}
