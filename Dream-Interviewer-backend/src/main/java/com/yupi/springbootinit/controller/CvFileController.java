package com.yupi.springbootinit.controller;


import com.yupi.springbootinit.model.dto.context.request.ContextRequest;
import com.yupi.springbootinit.service.impl.CvFileService;
import com.yupi.springbootinit.service.impl.SessionMessageService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.multipart.MultipartFile;

import javax.servlet.http.HttpServletRequest;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Random;



@RestController
public class CvFileController {

    //@Value("${file.upload.path}")
    private String uploadPath = "";



    @PostMapping("/upload")
    public ResponseEntity<String> uploadFile(@RequestParam("cvfile")  MultipartFile file ,HttpServletRequest request, ContextRequest contextRequest) {

        System.out.println("上传文件ing");
        /*ResponseEntity<String> responseEntity = new ResponseEntity<>("上传成功", HttpStatus.OK);
        return responseEntity;*/
        // 创建RestTemplate实例
        RestTemplateBuilder restTemplate = new RestTemplateBuilder();
        return new CvFileService(restTemplate).uploadfile(file, request);
    }
}
