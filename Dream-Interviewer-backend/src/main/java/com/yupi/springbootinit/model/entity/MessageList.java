package com.yupi.springbootinit.model.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.io.Serializable;
import java.util.Date;

/**
 * 
 * @TableName message_list
 */
@TableName(value ="message_list")
@Data
public class MessageList implements Serializable {
    /**
     * 
     */
    @TableId(type = IdType.ASSIGN_ID)
    private Integer listId;

    /**
     * 
     */
    private Date createTime;

    /**
     * 
     */
    private String userAccount;


    private String mainContent;

    /**
     * 
     */
    @TableLogic
    private Integer isDelete;

    @TableField(exist = false)
    private static final long serialVersionUID = 1L;
}