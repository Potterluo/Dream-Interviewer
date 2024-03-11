package com.yupi.springbootinit.model.dto.context.request;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Component
@ConfigurationProperties(prefix = "global")
public class GlobalMessageList {
    private String messagelistid;

    public String getVariable() {
        return messagelistid;
    }

    public void setVariable(String messagelistid) {
        this.messagelistid = messagelistid;
    }
}