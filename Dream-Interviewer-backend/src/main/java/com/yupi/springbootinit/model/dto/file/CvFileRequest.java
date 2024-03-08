package com.yupi.springbootinit.model.dto.file;

import org.springframework.web.multipart.MultipartFile;

import java.util.Date;

public class CvFileRequest {
    public long lastModified;
    public Date lastModifiedDate;

    public long getLastModified() {
        return lastModified;
    }

    public void setLastModified(long lastModified) {
        this.lastModified = lastModified;
    }

    public Date getLastModifiedDate() {
        return lastModifiedDate;
    }

    public void setLastModifiedDate(Date lastModifiedDate) {
        this.lastModifiedDate = lastModifiedDate;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public MultipartFile getOriginFileObj() {
        return originFileObj;
    }

    public void setOriginFileObj(MultipartFile originalFilename) {
        this.originFileObj = originalFilename;
    }

    public String name;
    public MultipartFile originFileObj;

}
