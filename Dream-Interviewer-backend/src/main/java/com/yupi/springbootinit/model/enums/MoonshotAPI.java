package com.yupi.springbootinit.model.enums;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

@Component
public class MoonshotAPI {
    public String getAPI_KEY() {
        return API_KEY;
    }

    public void setAPI_KEY(String API_KEY) {
        this.API_KEY = API_KEY;
    }

    public String getBASE_URL() {
        return BASE_URL;
    }

    public void setBASE_URL(String BASE_URL) {
        this.BASE_URL = BASE_URL;
    }

    @Value("${moonshot.key}")
    private String API_KEY;
    @Value("${moonshot.url}")
    private String BASE_URL;
}
