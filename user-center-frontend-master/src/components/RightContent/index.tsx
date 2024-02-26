import {Space, Switch} from 'antd';
import React from 'react';
import { useModel } from 'umi';
import styles from './index.less';
import AvatarDropdown from "./AvatarDropdown";
export type SiderTheme = 'light' | 'dark';
import {disable as darkreaderDisable, enable as darkreaderEnable, setFetchMethod as setFetch} from "@umijs/ssr-darkreader";

const updateDarkMode = (dark: boolean | undefined) =>{

  if (typeof window === 'undefined') return;
  if (typeof window.MutationObserver === 'undefined') return;

  if (dark) {
    const defaultTheme = {
      brightness: 100,
      contrast: 90,
      sepia: 10,
    };

    const defaultFixes = {
      invert: [],
      css: '',
      ignoreInlineStyle: ['.react-switch-handle'],
      ignoreImageAnalysis: [],
      disableStyleSheetsProxy: true,
    }
    if (window.Mutation0bserver && window.fetch) {
      setFetch(window.fetch);
      darkreaderEnable(defaultTheme, defaultFixes)
    }
  } else{
    if (window.Mutation0bserver) darkreaderDisable();
  }
}
const GlobalHeaderRight: React.FC = () => {
  const { initialState } = useModel('@@initialState');

  if (!initialState || !initialState.settings) {
    return null;
  }

  const { navTheme, layout } = initialState.settings;
  let className = styles.right;

  if ((navTheme === 'dark' && layout === 'top') || layout === 'mix') {
    className = `${styles.right}  ${styles.dark}`;
  }



  let darkMode = localStorage.getItem("darkMode") == "1";
  //更新一下黑暗模式的状态
  updateDarkMode(darkMode);

  const switchDarkMode = () => {
    //点击开关触发主题的切换
    localStorage.setItem("darkMode", darkMode ? "0" : "1");
    /*alert("切换");*/
    darkMode = !darkMode;
    updateDarkMode(darkMode); //这里是调用上面复制的updataTheme方法
  };


  return (
    <Space className={className}>
      <Switch checkedChildren = '🌙' unCheckedChildren = '☀️' onClick={switchDarkMode} defaultChecked={darkMode} ></Switch>
      <AvatarDropdown menu />
    </Space>
  );
};

export default GlobalHeaderRight;
