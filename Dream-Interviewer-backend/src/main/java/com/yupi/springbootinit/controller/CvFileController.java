package com.yupi.springbootinit.controller;


import com.yupi.springbootinit.model.dto.context.request.ContextRequest;
import com.yupi.springbootinit.service.CvFileService;
import com.yupi.springbootinit.service.impl.CvFileServiceImpl;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

import javax.servlet.http.HttpServletRequest;


@RestController
public class CvFileController {

    //@Value("${file.upload.path}")
    private String uploadPath = "";

    @Autowired
    CvFileServiceImpl cvFileServiceImpl;


    @PostMapping("/upload")
    public ResponseEntity<String> uploadFile(@RequestParam("cvfile")  MultipartFile file ,HttpServletRequest request, ContextRequest contextRequest) {

        System.out.println("上传文件ing");
        /*ResponseEntity<String> responseEntity = new ResponseEntity<>("上传成功", HttpStatus.OK);
        return responseEntity;*/
        // 创建RestTemplate实例
        RestTemplateBuilder restTemplate = new RestTemplateBuilder();

        return cvFileServiceImpl.uploadfile(file, request);
    }
}
