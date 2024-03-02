export default [
  { path: '/user', layout: false, routes: [
    { path: '/user/login', component: './User/Login' },
      { path: '/user/register', component: './User/Register' }
    ] },
  { path: '/welcome', layout: false,icon: 'smile', component: './Welcome', name: "欢迎页" },
  { path: '/chat', icon: 'Android', component: './ChatPage', name: "面试问答" },
  {
    path: '/admin',
    icon: 'crown',
    name: "管理页",
    access: 'canAdmin',
    routes: [
      { path: '/admin', redirect: '/admin/user' },
      { icon: 'table', path: '/admin/user', component: './Admin/User', name: "用户管理" },
    ],
  },
  { path: '/', redirect: '/chat' },
  { path: '*', layout: false, component: './404' },
];
