import React, { useEffect, useState } from 'react';
import { Card, Button, Form, Input, message, Spin, Avatar, Descriptions, Divider, Typography, Space } from 'antd';
import { UserOutlined, LockOutlined, ArrowLeftOutlined, MailOutlined, PhoneOutlined, IdcardOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';

const { Title, Text } = Typography;

const ProfilePage = () => {
  const [userInfo, setUserInfo] = useState(null);
  const [loading, setLoading] = useState(true);
  const [pwdLoading, setPwdLoading] = useState(false);
  const [form] = Form.useForm();
  const navigate = useNavigate();

  useEffect(() => {
    // 获取个人信息
    setLoading(true);
    axios.get('/user/v1/info', {
      headers: { Authorization: 'Bearer ' + localStorage.getItem('token') }
    })
      .then(res => {
        setUserInfo(res.data.data);
      })
      .catch((err) => {
        console.error(err);
        message.error('获取个人信息失败');
      })
      .finally(() => setLoading(false));
  }, []);

  const onFinish = (values) => {
    setPwdLoading(true);
    axios.post('/user/v1/direct/modify', {
      userId: userInfo?.id,
      password: values.password
    }, {
      headers: { Authorization: 'Bearer ' + localStorage.getItem('token') }
    })
      .then(() => {
        message.success('密码修改成功，请重新登录');
        setTimeout(() => {
          localStorage.clear();
          navigate('/login');
        }, 1500);
      })
      .catch(err => {
        message.error(err.response?.data?.message || '密码修改失败');
      })
      .finally(() => setPwdLoading(false));
  };

  return (
    <div style={{ 
      minHeight: '100vh', 
      background: 'linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%)', 
      padding: '40px 20px', 
      display: 'flex', 
      justifyContent: 'center', 
      alignItems: 'center' 
    }}>
      <Card 
        style={{ 
          width: '100%', 
          maxWidth: 600, 
          borderRadius: 16, 
          boxShadow: '0 10px 25px rgba(0,0,0,0.08)',
          overflow: 'hidden'
        }}
        bodyStyle={{ padding: 40 }}
      >
        <div style={{ marginBottom: 24 }}>
            <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)}>返回</Button>
        </div>

        {loading ? (
          <div style={{ textAlign: 'center', padding: '40px 0' }}><Spin size="large" /></div>
        ) : userInfo && (
          <>
            <div style={{ textAlign: 'center', marginBottom: 40 }}>
              <Avatar 
                size={80} 
                src={userInfo.avatar}
                icon={<UserOutlined />} 
                style={{ backgroundColor: '#1890ff', marginBottom: 16, boxShadow: '0 4px 10px rgba(24, 144, 255, 0.3)' }} 
              />
              <Title level={3} style={{ margin: 0 }}>{userInfo.name || '用户'}</Title>
              <Text type="secondary">{userInfo.staffId || userInfo.username || 'Loading...'}</Text>
            </div>

            <Descriptions title="基本信息" column={1} bordered size="small" labelStyle={{ width: 120, fontWeight: 500 }}>
              <Descriptions.Item label={<Space><IdcardOutlined /> 姓名</Space>}>
                {userInfo.name}
              </Descriptions.Item>
            </Descriptions>
            
            <Divider style={{ margin: '32px 0' }} />
          </>
        )}
      </Card>
    </div>
  );
};

export default ProfilePage;
