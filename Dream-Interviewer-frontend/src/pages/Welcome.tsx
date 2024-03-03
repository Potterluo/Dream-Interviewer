import { PageContainer } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import {Button, Card, ConfigProvider, theme} from 'antd';
import React from 'react';
import {Helmet, Link} from "@@/exports";
import { Col, Row } from 'antd';
import Footer from "@/components/Footer";
import Settings from "../../config/defaultSettings";

/**
 * 每个单独的卡片，为了复用样式抽成了组件
 * @param param0
 * @returns
 */

const Welcome: React.FC = () => {
  // Inline CSS styles

  const buttonStyle = {
    backgroundColor: '#9e3ffd',
    color: 'white',
    border: 'none',
    height:'70px',
    width:'200px',
    fontSize: "28px",
    fontWeight:'bold',
    // 更多样式属性...
  };
  const buttonStyle1 = {
    backgroundColor: '#9e3ffd',
    color: 'white',
    border: 'none',
    height:'45px',
    width:'100px',
    fontSize: "24px",
    fontWeight:'bold',
    lineHeight:'normal'
    // 更多样式属性...
  };
  return (
    <div
    style={{
      backgroundImage:"url('https://pic.imgdb.cn/item/65e3ec999f345e8d03c4310f.png')",
      backgroundSize: '100% 100%',
      backgroundRepeat:"no-repeat",
      backgroundPosition:"center",
      /*backgroundSize:"cover",*/
      height:"100vh",
      flexDirection: 'column',
      position:"relative",
      display: 'flex',
      overflow: 'auto',

      /*border: "1px solid #66ccff"*/
    }}>
      <Helmet>
        <title>
          {'欢迎'}- {Settings.title}
        </title>
      </Helmet>
      <div style={{
        flex:'1',
        padding:'12px 0'
      }}>
        <Row>
          <Col span={24}>
            <div style={{
              height:"30px"
            }}></div>
          </Col>
        </Row>
        <Row>
          <Col span={22}></Col>
          <Col span={2}>
            <Link to="/user/login">
              <Button type="primary" shape="round" size={"large"} style={buttonStyle1} >
                Login
              </Button>
            </Link></Col>
        </Row>
        <Row>
          <Col span={1}></Col>
          <Col span={23}>
            <p style={{
              fontSize: "90px",
              fontFamily: "黑体",
              fontWeight: "bold",
              marginBottom:"10px",

            }}>Dream Interviewer</p>
            <p style={{
              fontSize: "40px",
              fontFamily: "黑体",
            }}>小梦面试官，你的面试好帮手！</p>
            <Link to="/user/login">
              <Button type="primary" shape="round" size={"large"} style={buttonStyle} >
                Get Start
              </Button>
            </Link>
          </Col>

        </Row>
      </div>
      <Footer />

    </div>


  );
};

export default Welcome;
