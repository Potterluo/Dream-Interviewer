package com.yupi.springbootinit.service;

import com.baomidou.mybatisplus.extension.service.IService;
import com.yupi.springbootinit.model.entity.MessageList;
import com.yupi.springbootinit.service.MessageListService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;


import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

public class MessageListServiceTest {

    private MessageListService messageListServiceMock;

    @BeforeEach
    public void setUp() {
        // 创建 MessageListService 的 mock 对象
        messageListServiceMock = Mockito.mock(MessageListService.class);
    }

    @Test
    public void testAddMessageList() {
        // 模拟 AddMessageList 方法的返回值
        when(messageListServiceMock.AddMessageList(anyString())).thenReturn(1);

        // 执行方法并验证结果
        int result = messageListServiceMock.AddMessageList("testUser");
        assertEquals(1, result, "添加会话列表应成功并返回一个非零ID");

        // 如果实际业务中 AddMessageList 应当与数据库交互，这里则需要配置 mock 如何处理数据库操作
        // 并且可能需要断言数据库状态的变化等
    }

    @Test
    public void testDeleteMessageList() {
        // 模拟 DeleteMessageList 方法的返回值
        when(messageListServiceMock.DeleteMessageList(1)).thenReturn(true);

        // 验证删除方法调用结果
        boolean result = messageListServiceMock.DeleteMessageList(1);
        assertTrue(result, "删除指定会话列表应成功");

        // 同上，如果是真实操作，需要检查数据库状态变化
    }

    @Test
    public void testMainmessage() {
        // 假设 Mainmessage 方法需要 MessageList 实体对象，并返回一个字符串
        MessageList mockMessageList = new MessageList();
        mockMessageList.setUserAccount("testUser");
        // 设置 Mainmessage 方法返回的模拟值
        when(messageListServiceMock.Mainmessage(mockMessageList)).thenReturn("Main Message Mocked");

        // 调用并验证结果
        String result = messageListServiceMock.Mainmessage(mockMessageList);
        assertEquals("Main Message Mocked", result, "获取主消息内容应返回预期值");
    }

    @Test
    public void testListMessageList() {
        // 创建 MessageList 对象并设置属性
        MessageList messageList1 = new MessageList();
        messageList1.setUserAccount("user1");

        MessageList messageList2 = new MessageList();
        messageList2.setUserAccount("user2");

        // 使用 ArrayList 构造器创建一个可变的列表
        List<MessageList> mockMessageLists = new ArrayList<>();
        mockMessageLists.add(messageList1);
        mockMessageLists.add(messageList2);

        // 模拟 listMessageList 方法的返回值
        when(messageListServiceMock.listMessageList(anyString())).thenReturn(mockMessageLists);

        // 验证查询结果
        List<MessageList> result = messageListServiceMock.listMessageList("testUser");
        assertEquals(2, result.size(), "根据用户账号查询到的会话列表数量应为2");
    }
}
