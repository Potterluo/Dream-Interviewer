package com.yupi.springbootinit.service.impl;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.yupi.springbootinit.model.dto.context.response.FileContentResponse;
import com.yupi.springbootinit.model.dto.context.response.UploadFileResopnse;
import com.yupi.springbootinit.model.enums.FilePath;
import com.yupi.springbootinit.model.enums.MoonshotAPI;
import com.yupi.springbootinit.service.CvFileService;
import org.springframework.beans.factory.annotation.Autowired;
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
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Duration;
import java.util.Random;

import com.fasterxml.jackson.databind.ObjectMapper;

import javax.servlet.http.HttpServletRequest;

@Service
public class CvFileServiceImpl
implements CvFileService {

    // @Value("${moonshot.api.key}")

    @Autowired
    private  MoonshotAPI moonshotAPI;

    @Autowired
    private FilePath filePath;
    //TODO 上传时修改
/*    String uploadPath = "E:\\TestUpload";*/

    private static final RestTemplate restTemplate;




    static {
        RestTemplateBuilder restTemplateBuilder = new RestTemplateBuilder();
        restTemplate = restTemplateBuilder
                .setConnectTimeout(Duration.ofSeconds(30))
                .setReadTimeout(Duration.ofSeconds(30))
                .build();
    }

    public CvFileServiceImpl() {
    }

    public String getFileContent(String fileId) {
        // 构建请求URL
        String url = moonshotAPI.getBASE_URL()+"/files/" + fileId + "/content";
        // 设置请求头
        HttpHeaders headers = new HttpHeaders();
        headers.set("Authorization", "Bearer " + moonshotAPI.getAPI_KEY());
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
    public  String uploadMSAiFiles(File file) throws JsonProcessingException {


        // 创建RestTemplate实例
        RestTemplate restTemplate = new RestTemplate();


        // 设置请求头
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.MULTIPART_FORM_DATA);
        headers.set("Authorization", "Bearer " + moonshotAPI.getAPI_KEY());

        // 创建多部分请求体
        MultiValueMap<String, Object> body = new LinkedMultiValueMap<>();
        body.add("file", new FileSystemResource(file)); // 将MultipartFile转换为FileSystemResource添加到请求体
        body.add("purpose", "file-extract");

        // 创建请求实体
        HttpEntity<MultiValueMap<String, Object>> request = new HttpEntity<>(body, headers);

        // 发送POST请求
        String MOONSHOT_FILE_API_BASE_URL = moonshotAPI.getBASE_URL() + "/files";
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


    public ResponseEntity<String> uploadfile(@RequestParam("cvfile")  MultipartFile file , HttpServletRequest request) {
        if (file.isEmpty()) {
            return new ResponseEntity<>("请选择文件", HttpStatus.BAD_REQUEST);
        }

        try {
            String fileName = String.format("%06d", new Random().nextInt(900000) + 1) + file.getOriginalFilename();
            // 设置文件存储路径
            Path path = Paths.get(filePath.getUploadPath(), fileName);
            // 保存文件
            Files.copy(file.getInputStream(), path);
            //TODO 文件路径改写
            File upfile = new File(filePath.getUploadPath() + fileName);
            //CvFileServiceImpl cvFileService = new CvFileServiceImpl();
            // 上传文件至Moonshot
            String fileId = this.uploadMSAiFiles(upfile);
            //抽取文件内容
            String fileContent = this.getFileContent(fileId);
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
