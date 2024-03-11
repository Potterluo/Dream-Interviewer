package com.yupi.springbootinit.model.dto.context.request;

import com.yupi.springbootinit.model.dto.moonshotai.PostMessages;
import com.yupi.springbootinit.model.dto.moonshotai.PostSendRequest;

import java.util.List;

public class SessionMessageRequest {
    long listId;
    String userAccount;
    String model;

    public long getListId() {
        return listId;
    }

    public void setListId(long listId) {
        this.listId = listId;
    }

    public String getUserAccount() {
        return userAccount;
    }

    public void setUserAccount(String userAccount) {
        this.userAccount = userAccount;
    }

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }

    public List<PostMessages> getMessagesList() {
        return messagesList;
    }

    public void setMessagesList(List<PostMessages> messagesList) {
        this.messagesList = messagesList;
    }

    List<PostMessages> messagesList;

    private static final long serialVersionUID = 1L; // 序列化版本号

    public PostSendRequest getPostSendRequest() {
        PostSendRequest postSendRequest = new PostSendRequest();
        postSendRequest.setModel(this.getModel());
        postSendRequest.setMessages(this.getMessagesList());
        return postSendRequest;
    }
}
