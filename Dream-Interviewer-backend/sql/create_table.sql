# 数据库初始化

-- 创建库
create database if not exists interviewer_db;

-- 切换库
use dream_interviewer_db;

create table context
(
    id               int          not null comment '对话id'
        primary key,
    timeStamp        datetime     not null on update CURRENT_TIMESTAMP comment '产生时间戳',
    userAccount      varchar(255) not null comment '发起会话的用户',
    messageRole      varchar(255) not null comment 'assistant/user，对话所属',
    messageContent   varchar(255) not null comment '会话内容',
    ListId           bigint       not null comment '所属会话列表',
    messageModel     varchar(255) not null comment '8k,32k,128k',
    promptTokens     int          null comment '提示词所消耗的tokens',
    completionTokens int          null comment '回复所消耗的tokens',
    totalTokens      int          null comment '会话所消耗的token'
)
    collate = latin1_swedish_ci;

create table message_list
(
    listId      int          not null
        primary key,
    createTime  datetime     not null on update CURRENT_TIMESTAMP,
    userAccount varchar(256) not null,
    isDelete    tinyint      not null
)
    collate = latin1_swedish_ci;

create table post
(
    id         bigint auto_increment comment 'id'
        primary key,
    title      varchar(512)                       null comment '标题',
    content    text                               null comment '内容',
    tags       varchar(1024)                      null comment '标签列表（json 数组）',
    thumbNum   int      default 0                 not null comment '点赞数',
    favourNum  int      default 0                 not null comment '收藏数',
    userId     bigint                             not null comment '创建用户 id',
    createTime datetime default CURRENT_TIMESTAMP not null comment '创建时间',
    updateTime datetime default CURRENT_TIMESTAMP not null on update CURRENT_TIMESTAMP comment '更新时间',
    isDelete   tinyint  default 0                 not null comment '是否删除'
)
    comment '帖子' collate = utf8mb4_unicode_ci;

create index idx_userId
    on post (userId);

create table post_favour
(
    id         bigint auto_increment comment 'id'
        primary key,
    postId     bigint                             not null comment '帖子 id',
    userId     bigint                             not null comment '创建用户 id',
    createTime datetime default CURRENT_TIMESTAMP not null comment '创建时间',
    updateTime datetime default CURRENT_TIMESTAMP not null on update CURRENT_TIMESTAMP comment '更新时间'
)
    comment '帖子收藏' collate = latin1_swedish_ci;

create index idx_postId
    on post_favour (postId);

create index idx_userId
    on post_favour (userId);

create table post_thumb
(
    id         bigint auto_increment comment 'id'
        primary key,
    postId     bigint                             not null comment '帖子 id',
    userId     bigint                             not null comment '创建用户 id',
    createTime datetime default CURRENT_TIMESTAMP not null comment '创建时间',
    updateTime datetime default CURRENT_TIMESTAMP not null on update CURRENT_TIMESTAMP comment '更新时间'
)
    comment '帖子点赞' collate = latin1_swedish_ci;

create index idx_postId
    on post_thumb (postId);

create index idx_userId
    on post_thumb (userId);

create table user
(
    id           bigint auto_increment comment 'id'
        primary key,
    userAccount  varchar(256)                           not null comment '账号',
    userPassword varchar(512)                           not null comment '密码',
    unionId      varchar(256)                           null comment '微信开放平台id',
    mpOpenId     varchar(256)                           null comment '公众号openId',
    userName     varchar(256)                           null comment '用户昵称',
    userAvatar   varchar(1024)                          null comment '用户头像',
    userProfile  varchar(512)                           null comment '用户简介',
    userRole     varchar(256) default 'user'            not null comment '用户角色：user/admin/ban',
    createTime   datetime     default CURRENT_TIMESTAMP not null comment '创建时间',
    updateTime   datetime     default CURRENT_TIMESTAMP not null on update CURRENT_TIMESTAMP comment '更新时间',
    isDelete     tinyint      default 0                 not null comment '是否删除'
)
    comment '用户' collate = utf8mb4_unicode_ci;

create index idx_unionId
    on user (unionId);

create index userAccount
    on user (userAccount);

