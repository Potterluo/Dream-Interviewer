package com.yupi.springbootinit.model.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import java.io.Serializable;
import java.util.Date;
import lombok.Data;

/**
 * 
 * @TableName context
 */
@TableName(value ="context")
@Data
public class Context implements Serializable {
    /**
     * 对话id
     */
    @TableId(type = IdType.ASSIGN_ID)
    private Integer id;

    /**
     * 产生时间戳
     */
    private Date timeStamp;

    /**
     * 发起会话的用户
     */
    private String userAccount;

    /**
     * assistant/user，对话所属
     */
    private String messageRole;

    /**
     * 会话内容
     */
    private String messageContent;

    /**
     * 所属会话列表
     */
    private Long listId;

    /**
     * 8k,32k,128k
     */
    private String messageModel;

    /**
     * 提示词所消耗的tokens
     */
    private Integer promptTokens;

    /**
     * 回复所消耗的tokens
     */
    private Integer completionTokens;

    /**
     * 会话所消耗的token
     */
    private Integer totalTokens;

    @TableField(exist = false)
    private static final long serialVersionUID = 1L;
}