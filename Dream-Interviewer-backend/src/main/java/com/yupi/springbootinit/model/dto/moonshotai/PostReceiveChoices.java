package com.yupi.springbootinit.model.dto.moonshotai;

/*"choices": [
        {
        "index": 0,
        "message": {
        "role": "assistant",
        "content": " 我是MoonshotAI，一个由Moonshot Corp开发的人工智能助手。我可以帮助您回答问题和解决问题。请问有什么我可以帮助您的？"
        },
        "finish_reason": "stop"
        }
        ],*/
public class PostReceiveChoices {
    public int getIndex() {
        return index;
    }

    public void setIndex(int index) {
        this.index = index;
    }

    public PostMessages getMessage() {
        return message;
    }

    public void setMessage(PostMessages message) {
        this.message = message;
    }

    int index;
    PostMessages message;

    public String getFinish_reason() {
        return finish_reason;
    }

    public void setFinish_reason(String finish_reason) {
        this.finish_reason = finish_reason;
    }

    String finish_reason;
}
