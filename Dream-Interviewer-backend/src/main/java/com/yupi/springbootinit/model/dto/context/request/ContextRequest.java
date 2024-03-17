package com.yupi.springbootinit.model.dto.context.request;

import java.io.Serializable;

public class ContextRequest implements Serializable {
    public String getUserAccount() {
        return userAccount;
    }

    public void setUserAccount(String userAccount) {
        this.userAccount = userAccount;
    }

    public String getMessageRole() {
        return messageRole;
    }

    public void setMessageRole(String messageRole) {
        this.messageRole = messageRole;
    }

    public String getMessageContent() {
        return messageContent;
    }

    public void setMessageContent(String messageContent) {
        this.messageContent = messageContent;
    }

    public long getListId() {
        return ListId;
    }

    public void setListId(long listId) {
        ListId = listId;
    }

    String userAccount;
    String messageRole;
    String messageContent;
    long ListId;
    private static final long serialVersionUID = 1L; // 序列化版本号
}
