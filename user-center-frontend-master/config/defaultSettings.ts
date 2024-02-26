import { Settings as LayoutSettings } from '@ant-design/pro-layout';

const Settings: LayoutSettings & {
  pwa?: boolean;
  logo?: string;
} = {
  navTheme: 'realDark',
  // 拂晓蓝
  primaryColor: '#52C41A',
  layout: 'top',
  contentWidth: 'Fixed',
  fixedHeader: false,
  fixSiderbar: true,
  colorWeak: false,
  title: '小梦面试官',
  pwa: false,
  logo: 'https://pic.imgdb.cn/item/65db228f9f345e8d03c78586.png',
  iconfontUrl: '',
  "headerHeight": 48,
  "splitMenus": false,
  "footerRender": false
};

export default Settings;

/*{
  "navTheme": "realDark",
  "primaryColor": "#52C41A",
  "layout": "top",
  "contentWidth": "Fixed",
  "fixedHeader": false,
  "fixSiderbar": true,
  "pwa": false,
  "logo": "https://pic.imgdb.cn/item/65db228f9f345e8d03c78586.png",
  "headerHeight": 48,
  "splitMenus": false,
  "footerRender": false
}*/
