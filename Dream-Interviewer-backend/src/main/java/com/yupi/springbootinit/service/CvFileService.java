package com.yupi.springbootinit.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import org.springframework.boot.web.client.RestTemplateBuilder;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.multipart.MultipartFile;

import javax.servlet.http.HttpServletRequest;
import java.io.File;

public interface CvFileService  {
    public String getFileContent(String fileId);

    public  String uploadMSAiFiles(File file) throws JsonProcessingException;

    public ResponseEntity<String> uploadfile(@RequestParam("cvfile") MultipartFile file , HttpServletRequest request);

}
