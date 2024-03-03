package com.yupi.springbootinit.model.dto.moonshotai;

import java.util.List;

public class PostSendRequest {
    String model;

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }



    public  double getTemperature() {
        return temperature;
    }

    public void setTemperature(double temperature) {
        this.temperature = temperature;
    }

    public List<PostSendMessages> getMessages() {
        return messages;
    }

    public void setMessages(List<PostSendMessages> messages) {
        this.messages = messages;
    }

    List<PostSendMessages> messages;
    double temperature;
}
