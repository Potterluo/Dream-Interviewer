package com.yupi.springbootinit.controller;


import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RequestPart;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;
import org.springframework.web.multipart.MultipartHttpServletRequest;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

@RestController
public class CvFileController {

    @Value("${file.upload.path}")
    private String uploadPath;

    @PostMapping("/upload")
    public ResponseEntity<String> uploadFile(@RequestParam("cvfile")  MultipartFile file) {
        if (file.isEmpty()) {
            return new ResponseEntity<>("请选择文件", HttpStatus.BAD_REQUEST);
        }

        try {
            String fileName = file.getOriginalFilename();
            // 设置文件存储路径
            Path path = Paths.get(uploadPath, fileName);
            // 保存文件
            Files.copy(file.getInputStream(), path);
            return new ResponseEntity<>("文件上传成功: " + fileName, HttpStatus.OK);
        } catch (Exception e) {
            e.printStackTrace();
            return new ResponseEntity<>("文件上传失败", HttpStatus.INTERNAL_SERVER_ERROR);
        }
        /*try {
            // 获取上传的文件
            MultipartFile file = request.getFile("cv");
            if (file.isEmpty()) {
                return new ResponseEntity<>("请选择文件上传", HttpStatus.BAD_REQUEST);
            }

            // 获取文件名
            String fileName = file.getOriginalFilename();
            // 创建文件路径
            Path path = Paths.get(uploadPath, fileName);

            // 保存文件到服务器
            Files.copy(file.getInputStream(), path);

            // 返回成功响应
            return new ResponseEntity<>("文件上传成功", HttpStatus.OK);
        } catch (IOException e) {
            // 处理异常
            e.printStackTrace();
            return new ResponseEntity<>("文件上传失败", HttpStatus.INTERNAL_SERVER_ERROR);
        }*/
    }
}
