declare namespace API {
  type addMessageListParams = {
    userAccount: string;
  };

  type BaseResponseBoolean = {
    code?: number;
    data?: boolean;
    message?: string;
  };

  type BaseResponseInteger = {
    code?: number;
    data?: number;
    message?: string;
  };

  type BaseResponseLoginUserVO = {
    code?: number;
    data?: LoginUserVO;
    message?: string;
  };

  type BaseResponseLong = {
    code?: number;
    data?: number;
    message?: string;
  };

  type BaseResponsePagePost = {
    code?: number;
    data?: PagePost;
    message?: string;
  };

  type BaseResponsePagePostVO = {
    code?: number;
    data?: PagePostVO;
    message?: string;
  };

  type BaseResponsePageUser = {
    code?: number;
    data?: PageUser;
    message?: string;
  };

  type BaseResponsePageUserVO = {
    code?: number;
    data?: PageUserVO;
    message?: string;
  };

  type BaseResponsePostVO = {
    code?: number;
    data?: PostVO;
    message?: string;
  };

  type BaseResponseString = {
    code?: number;
    data?: string;
    message?: string;
  };

  type BaseResponseUser = {
    code?: number;
    data?: User;
    message?: string;
  };

  type BaseResponseUserVO = {
    code?: number;
    data?: UserVO;
    message?: string;
  };

  type chat1Params = {
    userMessage: string;
  };

  type chat2Params = {
    sendMessage: PostSendRequest;
  };

  type checkParams = {
    timestamp: string;
    nonce: string;
    signature: string;
    echostr: string;
  };

  type Context = {
    id?: number;
    timeStamp?: string;
    userAccount?: string;
    messageRole?: string;
    messageContent?: string;
    listId?: number;
    messageModel?: string;
    promptTokens?: number;
    completionTokens?: number;
    totalTokens?: number;
  };

  type ContextRequest = {
    userAccount?: string;
    messageRole?: string;
    messageContent?: string;
    listId?: number;
  };

  type deleteMessageListParams = {
    listId: number;
  };

  type DeleteRequest = {
    id?: number;
  };

  type getPostVOByIdParams = {
    id: number;
  };

  type getUserByIdParams = {
    id: number;
  };

  type getUserVOByIdParams = {
    id: number;
  };

  type listContextParams = {
    listId: number;
  };

  type listMessageListParams = {
    userAccount: string;
  };

  type LoginUserVO = {
    id?: number;
    userName?: string;
    userAvatar?: string;
    userProfile?: string;
    userRole?: string;
    createTime?: string;
    updateTime?: string;
  };

  type mainMessageParams = {
    listId: number;
  };

  type MessageList = {
    listId?: number;
    createTime?: string;
    userAccount?: string;
    mainContent?: string;
    isDelete?: number;
  };

  type OrderItem = {
    column?: string;
    asc?: boolean;
  };

  type PagePost = {
    records?: Post[];
    total?: number;
    size?: number;
    current?: number;
    orders?: OrderItem[];
    optimizeCountSql?: boolean;
    searchCount?: boolean;
    optimizeJoinOfCountSql?: boolean;
    countId?: string;
    maxLimit?: number;
    pages?: number;
  };

  type PagePostVO = {
    records?: PostVO[];
    total?: number;
    size?: number;
    current?: number;
    orders?: OrderItem[];
    optimizeCountSql?: boolean;
    searchCount?: boolean;
    optimizeJoinOfCountSql?: boolean;
    countId?: string;
    maxLimit?: number;
    pages?: number;
  };

  type PageUser = {
    records?: User[];
    total?: number;
    size?: number;
    current?: number;
    orders?: OrderItem[];
    optimizeCountSql?: boolean;
    searchCount?: boolean;
    optimizeJoinOfCountSql?: boolean;
    countId?: string;
    maxLimit?: number;
    pages?: number;
  };

  type PageUserVO = {
    records?: UserVO[];
    total?: number;
    size?: number;
    current?: number;
    orders?: OrderItem[];
    optimizeCountSql?: boolean;
    searchCount?: boolean;
    optimizeJoinOfCountSql?: boolean;
    countId?: string;
    maxLimit?: number;
    pages?: number;
  };

  type Post = {
    id?: number;
    title?: string;
    content?: string;
    tags?: string;
    thumbNum?: number;
    favourNum?: number;
    userId?: number;
    createTime?: string;
    updateTime?: string;
    isDelete?: number;
  };

  type PostAddRequest = {
    title?: string;
    content?: string;
    tags?: string[];
  };

  type PostEditRequest = {
    id?: number;
    title?: string;
    content?: string;
    tags?: string[];
  };

  type PostFavourAddRequest = {
    postId?: number;
  };

  type PostFavourQueryRequest = {
    current?: number;
    pageSize?: number;
    sortField?: string;
    sortOrder?: string;
    postQueryRequest?: PostQueryRequest;
    userId?: number;
  };

  type PostMessages = {
    role?: string;
    content?: string;
  };

  type PostQueryRequest = {
    current?: number;
    pageSize?: number;
    sortField?: string;
    sortOrder?: string;
    id?: number;
    notId?: number;
    searchText?: string;
    title?: string;
    content?: string;
    tags?: string[];
    orTags?: string[];
    userId?: number;
    favourUserId?: number;
  };

  type PostSendRequest = {
    model?: string;
    messages?: PostMessages[];
    temperature?: number;
  };

  type PostThumbAddRequest = {
    postId?: number;
  };

  type PostUpdateRequest = {
    id?: number;
    title?: string;
    content?: string;
    tags?: string[];
  };

  type PostVO = {
    id?: number;
    title?: string;
    content?: string;
    thumbNum?: number;
    favourNum?: number;
    userId?: number;
    createTime?: string;
    updateTime?: string;
    tagList?: string[];
    user?: UserVO;
    hasThumb?: boolean;
    hasFavour?: boolean;
  };

  type uploadFile1Params = {
    uploadFileRequest: UploadFileRequest;
  };

  type UploadFileRequest = {
    biz?: string;
  };

  type User = {
    id?: number;
    userAccount?: string;
    userPassword?: string;
    unionId?: string;
    mpOpenId?: string;
    userName?: string;
    userAvatar?: string;
    userProfile?: string;
    userRole?: string;
    createTime?: string;
    updateTime?: string;
    isDelete?: number;
  };

  type UserAddRequest = {
    userName?: string;
    userAccount?: string;
    userAvatar?: string;
    userRole?: string;
  };

  type userLoginByWxOpenParams = {
    code: string;
  };

  type UserLoginRequest = {
    userAccount?: string;
    userPassword?: string;
  };

  type UserQueryRequest = {
    current?: number;
    pageSize?: number;
    sortField?: string;
    sortOrder?: string;
    id?: number;
    unionId?: string;
    mpOpenId?: string;
    userName?: string;
    userProfile?: string;
    userRole?: string;
  };

  type UserRegisterRequest = {
    userAccount?: string;
    userPassword?: string;
    checkPassword?: string;
  };

  type UserUpdateMyRequest = {
    userName?: string;
    userAvatar?: string;
    userProfile?: string;
  };

  type UserUpdateRequest = {
    id?: number;
    userName?: string;
    userAvatar?: string;
    userProfile?: string;
    userRole?: string;
  };

  type UserVO = {
    id?: number;
    userName?: string;
    userAvatar?: string;
    userProfile?: string;
    userRole?: string;
    createTime?: string;
  };
}
