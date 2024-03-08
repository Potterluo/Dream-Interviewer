import { history } from '@umijs/max';
import { Button, Col, Row } from 'antd';
import React from 'react';

const welcomePath = '/welcome';

const NoFoundPage: React.FC = () => (
  <div>
    <div
      style={{
        height: '10vh',
      }}
    ></div>
    <Row>
      <Col span={6}></Col>
      <Col span={4}>
        <h1
          style={{
            fontSize: '45px',
            fontWeight: 'bold',
            lineHeight: 1.5,
          }}
        >
          Uh oh! That page doesn't seem to exist
        </h1>
        <Button
          type="primary"
          style={{
            backgroundColor: '#7a6abf',
            color: 'white',
            border: 'none',
            height: '70px',
            width: '200px',
            fontSize: '20px',
            fontWeight: 'bold',
          }}
          onClick={() => history.push('/')}
        >
          Go to Homepage
        </Button>
      </Col>
      <Col span={2}></Col>
      <Col span={6}>
        {' '}
        <img src={'https://nulab.com/static/0367827d3653eb42e5dc96a5a32a3f3d/f5234/404.webp'} />
      </Col>
      <Col span={6}></Col>
    </Row>
  </div>
);

export default NoFoundPage;
