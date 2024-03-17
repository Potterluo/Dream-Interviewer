package com.yupi.springbootinit.model.dto.moonshotai;

import java.io.Serializable;

public class PostMessages implements Serializable {
    public String getRole() {
        return role;
    }

    public void setRole(String role) {
        this.role = role;
    }

    public String getContent() {
        return content;
    }

    public void setContent(String content) {
        this.content = content;
    }

    String role;
    String content;

    private static final long serialVersionUID = 1L; // 序列化版本号

}
