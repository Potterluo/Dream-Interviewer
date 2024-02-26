export default [
  {
    path: '/user',
    layout: false,
    routes: [
      {
        path: '/user', routes: [
          {name: '登录', path: '/user/login', component: './user/Login'},
          {name: '注册', path: '/user/register', component: './user/Register'}
        ]
      },
      {component: './404'},
    ],
  },
  {path: '/welcome', name: '欢迎', icon: 'smile', component: './Welcome'},
  {
    path: '/admin',
    name: '用户管理',
    icon: 'crown',
    access: 'canAdmin',
    component: './Admin/UserManage',
    /*routes: [
      {path: '/admin/user-manage', name: '用户管理', icon: 'smile', component: './Admin/UserManage'},
      {component: './404'},
    ],*/
  },
  /*{name: '查询表格', icon: 'table', path: '/list', component: './TableList'},*/
  {path: '/', redirect: '/welcome'},
  {component: './404'},
];
