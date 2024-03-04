package com.yupi.springbootinit.model.dto.moonshotai;

/*
返回类型
{
        "id": "cmpl-3f72922de0f84d1e9448b4e9d1f244c4",
        "object": "chat.completion",
        "created": 4255126,
        "model": "moonshot-v1-8k",
        "choices": [
        {
        "index": 0,
        "message": {
        "role": "assistant",
        "content": " 你好！我是 Kimi，由 Moonshot AI 提供的人工智能助手。我可以帮助回答问题、提供信息、协助解决问题，以及进行一些简单的对话。如果你有任何疑问或需要帮助，随时可以告诉我！"
        },
        "finish_reason": "stop"
        }
        ],
        "usage": {
        "prompt_tokens": 36,
        "completion_tokens": 45,
        "total_tokens": 81
        }
        }*/
public class PostReceiveRequest {
    String id;
    String object;
    long created;

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getObject() {
        return object;
    }

    public void setObject(String object) {
        this.object = object;
    }

    public long getCreated() {
        return created;
    }

    public void setCreated(long created) {
        this.created = created;
    }

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }

    public PostReceiveChoices[] getChoices() {
        return choices;
    }

    public void setChoices(PostReceiveChoices[] choices) {
        this.choices = choices;
    }

    public PostReceiveUsage getUsage() {
        return usage;
    }

    public void setUsage(PostReceiveUsage usage) {
        this.usage = usage;
    }

    String model;
    PostReceiveChoices[] choices;
    PostReceiveUsage usage;



}
