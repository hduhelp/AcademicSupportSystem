import React, { useState, useEffect, useRef } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { message, theme, Typography, Space, Avatar, Button, Modal, Drawer, Collapse, Tag, Image } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  LeftOutlined,
  RightOutlined,
  UserOutlined,
  RobotOutlined,
  EditOutlined,
  ShareAltOutlined,
  FileTextOutlined,
  ClockCircleOutlined,
  ReadOutlined,
  CaretRightOutlined,
  CloseOutlined,
  UpOutlined,
  DownOutlined
} from '@ant-design/icons';
import {
  Bubble,
  Sender,
  Conversations,
  Prompts,
  XProvider,
  Think,
} from '@ant-design/x';
import axios from 'axios';

// AI 头像地址
const AI_AVATAR = 'https://api.dicebear.com/7.x/bottts/svg?seed=Academic01';


import { chatCompletion, getHistories, getPaginationRecords, delHistory, initOutLinkChat } from '../api/fastgpt';
import { BASE_URL } from '../api/config';

// 解析 value 数组，返回结构化的内容列表
const parseMessageValue = (value) => {
  if (!Array.isArray(value)) {
    return [{ type: 'text', content: String(value || '') }];
  }
  
  return value.map(item => {
    if (item.type === 'text') {
       return { type: 'text', content: item.text?.content || '' };
    }
    if (item.type === 'interactive') {
       return { type: 'interactive', interactive: item.interactive };
    }
    if (item.type === 'reasoning') {
       return { type: 'reasoning', content: item.reasoning?.content || '' };
    }
    return null;
  }).filter(Boolean);
};

// 辅助函数
const getSafeContent = (val) => {
    if (typeof val === 'string') return val;
    if (typeof val === 'number') return String(val);
    if (!val) return '';
    if (typeof val === 'object') {
        if (val.value) return getSafeContent(val.value);
        if (val.content) return getSafeContent(val.content);
        return '';
    }
    return String(val);
};

// 引用详情抽屉
const ReferenceDrawer = ({ isOpen, onClose, quotes = [], startIndex = 0 }) => {
  const [currentIndex, setCurrentIndex] = useState(startIndex);
  
  useEffect(() => {
      setCurrentIndex(startIndex);
  }, [startIndex, isOpen]);

  if (!quotes || quotes.length === 0) return null;

  const handlePrev = () => {
     setCurrentIndex(prev => (prev > 0 ? prev - 1 : quotes.length - 1));
  };
  
  const handleNext = () => {
     setCurrentIndex(prev => (prev < quotes.length - 1 ? prev + 1 : 0));
  };

  const currentQuote = quotes[currentIndex] || {};
  const safeSourceName = getSafeContent(currentQuote.sourceName);
  const safeTitle = getSafeContent(currentQuote.title);
  const safeBody = getSafeContent(currentQuote.q || currentQuote.content || '无内容');

  return (
       <Drawer
          title={null}
          placement="right"
          closable={false}
          onClose={onClose}
          open={isOpen}
          width={600}
          styles={{ body: { padding: 0 }, header: { display: 'none' } }}
       >
          {/* Header */}
          <div style={{ padding: '16px 24px', borderBottom: '1px solid #f0f0f0', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', flex: 1, overflow: 'hidden', marginRight: 16 }}>
                  <FileTextOutlined style={{ fontSize: 20, color: '#ff4d4f', marginRight: 12 }} />
                  <Typography.Text ellipsis strong style={{ fontSize: 16 }}>
                      {safeSourceName || "引用详情"}
                  </Typography.Text>
              </div>
              <Button type="text" icon={<CloseOutlined />} onClick={onClose} />
          </div>

          {/* Navigation & Meta Bar */}
           <div style={{ padding: '12px 24px', background: '#fafafa', borderBottom: '1px solid #f0f0f0', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
               <Space>
                   <Tag>引用 {currentIndex + 1} / {quotes.length}</Tag>
                   {currentQuote?.score && <Tag color="green">综合排名: {getSafeContent(currentQuote.score)}</Tag>}
               </Space>
               
               <Space>
                   <Button size="small" icon={<LeftOutlined />} onClick={handlePrev} disabled={quotes.length <= 1}/>
                   <Button size="small" icon={<RightOutlined />} onClick={handleNext} disabled={quotes.length <= 1}/>
               </Space>
           </div>
           
           {/* Disclaimer */}
           <div style={{ padding: '8px 24px', background: '#fffbe6', fontSize: 12, color: '#faad14' }}>
               此处仅显示实际引用内容，若数据有更新，此处不会实时更新
           </div>

           {/* Content */}
           <div style={{ padding: '24px', overflowY: 'auto', height: 'calc(100% - 130px)' }}>
                {safeTitle && <Typography.Title level={4} style={{marginTop: 0}}>{safeTitle}</Typography.Title>}
                
               <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  components={{
                      img: ({node, ...props}) => {
                         let src = props.src;
                         // Handle relative API paths by prepending backend URL
                         if (src && src.startsWith('/api')) {
                             src = `${BASE_URL}${src}`;
                         }
                         return (
                            <Image 
                              {...props} 
                              src={src} 
                              style={{ maxWidth: '100%', borderRadius: 8, margin: '8px 0', objectFit: 'contain' }} 
                            />
                         );
                      },
                      p: ({node, ...props}) => <Typography.Paragraph {...props} />,
                      h1: ({node, ...props}) => <Typography.Title level={3} {...props} />,
                      h2: ({node, ...props}) => <Typography.Title level={4} {...props} />,
                      h3: ({node, ...props}) => <Typography.Title level={5} {...props} />,
                      code: ({node, inline, className, children, ...props}) => {
                          const match = /language-(\w+)/.exec(className || '');
                          return !inline ? (
                            <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4 }}>
                               <code className={className} {...props}>
                                  {children}
                               </code>
                            </pre>
                          ) : (
                            <code style={{ background: '#f5f5f5', padding: '2px 4px', borderRadius: 2 }} {...props}>
                                {children}
                            </code>
                          );
                      }
                  }}
              >
                  {safeBody}
              </ReactMarkdown>
           </div>
       </Drawer>
  );
};

// 引用列表组件
const ReferenceList = ({ quotes, onOpen }) => {
  const [expanded, setExpanded] = useState(false);

  if (!quotes || quotes.length === 0) return null;

  const handleOpenQuote = (index) => {
    onOpen && onOpen(quotes, index);
  };

  const visibleQuotes = expanded ? quotes : quotes.slice(0, 1);

  return (
    <div style={{ marginTop: 8 }}>
       <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
           {visibleQuotes.map((q, idx) => (
              <div 
                 key={idx} 
                 style={{ 
                    cursor: 'pointer', 
                    padding: '8px', 
                    background: '#f9f9f9', 
                    borderRadius: 4, 
                    border: '1px solid #eee',
                    display: 'flex',
                    alignItems: 'center',
                    fontSize: 12
                 }}
                 onClick={() => handleOpenQuote(idx)}
              >
                  <FileTextOutlined style={{ marginRight: 8, color: '#1890ff' }} />
                  <Typography.Text ellipsis style={{ flex: 1, color: '#1890ff' }}>
                      {getSafeContent(q.sourceName) || `Reference ${idx + 1}`}
                  </Typography.Text>
              </div>
           ))}
           {quotes.length > 1 && (
               <div 
                 style={{ 
                    fontSize: 12, 
                    color: '#999', 
                    cursor: 'pointer', 
                    paddingLeft: 8,
                    marginTop: 4
                 }}
                 onClick={() => setExpanded(!expanded)}
               >
                   {expanded ? '收起引用' : `查看全部 ${quotes.length} 条引用`}
               </div>
           )}
       </div>
    </div>
  );
};


// 交互式选项组件
const InteractiveOptions = ({ interactive, onSelect }) => {
  if (!interactive || interactive.type !== 'userSelect') return null;

  const { params } = interactive;
  if (!params) return null;

  const { description, userSelectOptions, userSelectedVal } = params;

  // 如果已经选过了，就只显示选中的那个（或者都不显示，根据设计。这里可以只显示结果）
  // 用户需求：选择后发送消息。
  
  if (userSelectedVal) {
     return (
        <div style={{ marginTop: 8, padding: '12px', background: '#f5f5f5', borderRadius: 8 }}>
            <Typography.Text type="secondary">已选择: {userSelectedVal}</Typography.Text>
        </div>
     );
  }

  return (
    <div style={{ 
        marginTop: 8, 
        padding: 0, 
        background: '#fff', 
        borderRadius: 8, 
        border: '1px solid #dae0e6',
        overflow: 'hidden',
        boxShadow: '0 2px 8px rgba(0,0,0,0.05)'
    }}>
      {description && (
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #f0f0f0', background: '#fafafa' }}>
           <Typography.Text strong>{description}</Typography.Text>
        </div>
      )}
      <div style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          {userSelectOptions?.map((option) => (
            <Button 
                key={option.key} 
                block 
                onClick={() => onSelect?.(option.value)}
                style={{ textAlign: 'left' }}
            >
              {option.value}
            </Button>
          ))}
      </div>
    </div>
  );
};


// 打字机效果的 Markdown 渲染组件
const TypingMarkdown = ({ content, isTyping, style = {}, sourceList = [], onReferenceClick }) => {
  const [displayedContent, setDisplayedContent] = useState(isTyping ? '' : content);
  const [showCursor, setShowCursor] = useState(true);
  const contentRef = useRef(content);

  useEffect(() => {
    contentRef.current = content;
  }, [content]);

  // Cursor blinking
  useEffect(() => {
     if (!isTyping) return;
     const cursorInterval = setInterval(() => {
        setShowCursor(prev => !prev);
     }, 500);
     return () => clearInterval(cursorInterval);
  }, [isTyping]);

  // Typing logic
  useEffect(() => {
    if (!isTyping) {
        setDisplayedContent(content);
        return;
    }

    const interval = setInterval(() => {
        setDisplayedContent((prev) => {
             const fullContent = contentRef.current;
             if (prev.length < fullContent.length) {
                 const diff = fullContent.length - prev.length;
                 // 动态调整打字速度，防止落后太多
                 // 调整为更慢的速度：基础间隔 50ms，并不容易触发加速
                 const step = diff > 200 ? 3 : (diff > 100 ? 2 : 1);
                 return fullContent.slice(0, prev.length + step);
             }
             return prev;
        });
    }, 50);
    
    return () => clearInterval(interval);
  }, [isTyping]);

  // 当 typing 状态结束时，确保显示完整内容
  useEffect(() => {
      if(!isTyping) {
          setDisplayedContent(content);
      }
  }, [isTyping, content]);

  return (
    <div className="markdown-body" style={{ whiteSpace: 'normal', wordBreak: 'break-word', fontSize: 14, ...style }}>
       <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
              img: ({node, ...props}) => {
                 let src = props.src;
                 if (src && src.startsWith('/api')) {
                     src = `${BASE_URL}${src}`;
                 }
                 return (
                    <Image 
                      {...props} 
                      src={src} 
                      style={{ maxWidth: '100%', borderRadius: 8, margin: '8px 0', objectFit: 'contain' }} 
                    />
                 );
              },
              p: ({node, ...props}) => <Typography.Paragraph {...props} style={{ marginBottom: '0.5em', fontSize: 'inherit' }} />,
              h1: ({node, ...props}) => <Typography.Title level={3} {...props} style={{ fontSize: '1.4em', marginTop: '0.5em' }} />,
              h2: ({node, ...props}) => <Typography.Title level={4} {...props} style={{ fontSize: '1.2em', marginTop: '0.5em' }} />,
              h3: ({node, ...props}) => <Typography.Title level={5} {...props} style={{ fontSize: '1.1em', marginTop: '0.5em' }} />,
              ul: ({node, ...props}) => <ul {...props} style={{ paddingLeft: 20, marginBottom: '0.5em' }} />,
              ol: ({node, ...props}) => <ol {...props} style={{ paddingLeft: 20, marginBottom: '0.5em' }} />,
              li: ({node, ...props}) => <li {...props} style={{ marginBottom: 4 }} />,
              a: ({node, href, children, ...props}) => {
                  if (href === 'CITE') {
                      const id = String(children);
                      let index = -1;
                      if (sourceList && sourceList.length > 0) {
                          // Try exact match id or _id
                          index = sourceList.findIndex(item => item.id === id || item._id === id);
                      }
                      
                      if (index !== -1) {
                         const displayIndex = index + 1;
                         return (
                            <sup 
                               style={{ 
                                  color: '#1890ff', 
                                  cursor: 'pointer', 
                                  fontWeight: 'bold',
                                  marginLeft: 2,
                                  fontSize: '0.75em'
                               }}
                               onClick={(e) => {
                                   e.stopPropagation();
                                   if (onReferenceClick) {
                                       onReferenceClick(index);
                                   }
                               }}
                            >
                               [{displayIndex}]
                            </sup>
                         );
                      }
                  }
                  return <a href={href} {...props}>{children}</a>;
              },
              code: ({node, inline, className, children, ...props}) => {
                  const match = /language-(\w+)/.exec(className || '');
                  return !inline ? (
                    <pre style={{ background: '#f5f5f5', padding: '12px', borderRadius: 6, overflowX: 'auto', marginBottom: 8, border: '1px solid #f0f0f0' }}>
                       <code className={className} {...props}>
                          {children}
                       </code>
                    </pre>
                  ) : (
                    <code style={{ background: '#f5f5f5', padding: '2px 4px', borderRadius: 4, color: '#eb5757', border: '1px solid #f0f0f0' }} {...props}>
                        {children}
                    </code>
                  );
              }
          }}
       >
           {displayedContent}
       </ReactMarkdown>
       {isTyping && (
           <span style={{
              display: 'inline-block',
              width: 3,
              height: 16,
              background: '#1677ff',
              marginLeft: 4,
              verticalAlign: 'text-bottom',
              opacity: showCursor ? 1 : 0,
              transition: 'opacity 0.2s',
              borderRadius: 1
           }} />
       )}
    </div>
  );
};

// 思考过程组件
const ThinkingProcess = ({ content, isTyping, duration }) => {
  const [activeKey, setActiveKey] = useState(isTyping ? ['1'] : []);

  useEffect(() => {
     if (isTyping) {
         setActiveKey(['1']);
     } else {
         setActiveKey([]);
     }
  }, [isTyping]);

  return (
    <div style={{ 
       backgroundColor: '#fff', 
       borderRadius: 6,
       marginBottom: 10,
       border: '1px solid rgba(0,0,0,0.08)',
       overflow: 'hidden'
    }}>
      <Collapse
         ghost
         activeKey={activeKey}
         onChange={(keys) => setActiveKey(keys)}
         expandIcon={({ isActive }) => <CaretRightOutlined rotate={isActive ? 90 : 0} style={{ fontSize: 12, color: 'rgba(0,0,0,0.25)' }}/>}
         items={[{
            key: '1',
            label: (
               <Space size={8}>
                  <Typography.Text style={{ fontSize: 12, color: 'rgba(0,0,0,0.65)' }}>思考过程</Typography.Text>
                  {duration && <Tag style={{ margin: 0, fontSize: 10, lineHeight: '16px', border: 0, background: 'rgba(0,0,0,0.06)' }}>{duration}s</Tag>}
               </Space>
            ),
            children: (
                <div style={{ padding: '0 4px 8px 12px' }}>
                    <TypingMarkdown 
                        content={content || ''} 
                        isTyping={isTyping} 
                        style={{ color: 'rgba(0, 0, 0, 0.45)', fontSize: 13 }} 
                    />
                </div>
            ),
         }]}
      />
    </div>
  );
};

const Chat = () => {
  const { token } = theme.useToken();
  const location = useLocation();
  const navigate = useNavigate();
  const { id, appId, shareId, title } = location.state || {}; 
  const outLinkUid = localStorage.getItem('staffId');

  const [histories, setHistories] = useState([]);
  const [activeChatId, setActiveChatId] = useState(null);
  const [messages, setMessages] = useState([]);
  const [sending, setSending] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const messagesEndRef = useRef(null);
  const ignoreLoadRef = useRef(null); // 用于标记跳过下一次 loadMessages 的 chatId
  
  const [referenceDrawerState, setReferenceDrawerState] = useState({
      open: false,
      quotes: [],
      index: 0
  });

  const [userInfo, setUserInfo] = useState(null);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (token) {
        axios.get('/user/v1/info', {
             headers: { Authorization: 'Bearer ' + token }
        }).then(res => {
             // 假设接口返回 { data: { avatar: 'url', ... } }
            setUserInfo(res.data.data);
        }).catch(err => {
            console.error('Fetch user info failed', err);
        });
    }
  }, []);

  const openReferenceDrawer = (quotes, index) => {
      setReferenceDrawerState({
          open: true,
          quotes: quotes || [],
          index: index || 0
      });
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // Initial Load
  useEffect(() => {
    if (!id) {
      message.error('缺少应用ID');
      navigate('/subjects');
      return;
    }
    loadHistories();
    handleNewChat();
  }, [id, navigate]);

  // Load messages when activeChatId changes
  useEffect(() => {
    if (activeChatId) {
      // 如果是被标记忽略的 chatId（通常是因为刚初始化且手动设置了欢迎语），则跳过本次加载
      if (ignoreLoadRef.current === activeChatId) {
          ignoreLoadRef.current = null;
          return;
      }
      loadMessages(activeChatId);
    } else {
      setMessages([]);
    }
  }, [activeChatId]);

  const loadHistories = async () => {
    try {
      const res = await getHistories({
        fastgptAppId: id,
        offset: 0,
        pageSize: 50,
        shareId: shareId || '',
        outLinkUid: outLinkUid || ''
      });
      const data = res.data?.data;
      const list = Array.isArray(data) ? data : (data?.list || []);
      setHistories(list);
    } catch (err) {
      console.error(err);
      message.error('加载历史会话失败');
    }
  };

  const loadMessages = async (chatId) => {
    try {
      const res = await getPaginationRecords({
        fastgptAppId: id,
        appId: appId,
        chatId,
        offset: 0,
        pageSize: 50,
        loadCustomFeedbacks: false
      });
      const data = res.data?.data;
      const records = Array.isArray(data) ? data : (data?.list || []);

      const mapped = records.map(r => {
        const parsedItems = parseMessageValue(r.value);
        return {
          key: r._id || r.dataId || Math.random().toString(),
          role: r.obj === 'Human' ? 'user' : 'ai',
          items: parsedItems,
          totalQuoteList: r.totalQuoteList || [],
          durationSeconds: r.durationSeconds,
          time: r.time,
        };
      });

      setMessages(mapped);
    } catch (err) {
      console.error(err);
      message.error('加载消息记录失败');
    }
  };

  const handleNewChat = async () => {
    setActiveChatId(null);
    setMessages([]);
    
    // 初始化新对话（如果需要立即创建后端记录的话）
    // 通常这里只生成ID，等到发送第一条消息时后端会自动处理，
    // 但如果有特殊初始化接口，可以在这里调用或者在发送前调用。
    // 按需求：点击新对话时调用接口
    if (shareId && outLinkUid) {
        const newChatId = Math.random().toString(36).substring(2, 15);
        try {
            const res = await initOutLinkChat(newChatId, shareId, outLinkUid);
            const data = res.data?.data;
            
            // 使用后端确认或返回的 chatId
            const finalChatId = data?.chatId || newChatId;
            
            // 如果后端返回了欢迎语，可以添加一条 AI 消息
            // 并且标记该 chatId 跳过下一次自动加载，防止被空的后端历史记录覆盖
            if (data?.app?.chatConfig?.welcomeText) {
                 ignoreLoadRef.current = finalChatId;
                 setMessages([{
                     key: Date.now().toString(),
                     role: 'ai',
                     items: [{ type: 'text', content: data.app.chatConfig.welcomeText }],
                     totalQuoteList: []
                 }]);
            }

            setActiveChatId(finalChatId);

            // 刷新历史记录以显示新会话
            loadHistories();
        } catch (err) {
            console.error('Failed to init chat', err);
            message.error('初始化新对话失败');
        }
    }
  };

  const handleDeleteHistory = async (chatId) => {
    try {
      await delHistory(id, chatId, shareId, outLinkUid);
      message.success('删除成功');
      setHistories(prev => prev.filter(h => h.chatId !== chatId));
      if (activeChatId === chatId) {
        setActiveChatId(null);
      }
    } catch (err) {
      message.error('删除失败');
    }
  };

  // Check if last message is interactive and pending
  const lastMsg = messages[messages.length - 1];
  const lastMsgLastItem = lastMsg?.items?.[lastMsg.items.length - 1];
  const isPendingInteractive = lastMsg?.role === 'ai' && lastMsgLastItem?.type === 'interactive' && !lastMsgLastItem.interactive?.params?.userSelectedVal;

  const onSend = async (val, interactiveParams = null) => {
    const text = typeof val === 'string' ? val : '';
    if (!text.trim()) return;
    
    // Optimistically update interactive state if this send is a response to it
    if (interactiveParams) {
        setMessages(prev => {
            const newArr = [...prev];
            const last = { ...newArr[newArr.length - 1] }; // Shallow copy msg
            if (last && last.items && last.items.length > 0) {
                // Find and update the interactive item
                const newItems = [...last.items];
                const lastItemIndex = newItems.length - 1;
                const lastItem = { ...newItems[lastItemIndex] };
                
                if (lastItem.type === 'interactive' && lastItem.interactive) {
                    lastItem.interactive = {
                        ...lastItem.interactive,
                        params: {
                            ...lastItem.interactive.params,
                            userSelectedVal: text
                        }
                    };
                    newItems[lastItemIndex] = lastItem;
                    last.items = newItems;
                    newArr[newArr.length - 1] = last;
                }
            }
            return newArr;
        });
    }

    const currentInput = text;
    setInputValue('');
    setSending(true);

    const newMsg = { 
        key: Date.now().toString(), 
        role: 'user', 
        items: [{ type: 'text', content: currentInput }],
        totalQuoteList: [],
        content: currentInput // legacy support for sending logic
    };

    const aiMsgKey = (Date.now() + 1).toString();
    const newAiMsg = { 
        key: aiMsgKey, 
        role: 'ai', 
        items: [{ type: 'text', content: '' }], 
        totalQuoteList: [] 
    };

    setMessages(prev => [...prev, newMsg, newAiMsg]);

    const targetChatId = activeChatId || Date.now().toString(); 
    if (!activeChatId) {
        setActiveChatId(targetChatId);
    }

    try {
      let requestMessages = [];
      
      if (interactiveParams) {
          requestMessages = [
              {
                  dataId: Math.random().toString(36).substring(2, 15), 
                  hideInUI: false,
                  role: 'user',
                  content: currentInput
              }
          ];
      } else {
          // Construct full context from existing messages
          // Note: parseMessageValue changed output structure, so we need to reconstruct plain text content for context if backend expects it.
          // Or backend expects structured 'value' array? 
          // FastGPT usually supports 'content' string. 
          // We need to flatten the items to string for context.
          
          requestMessages = [
            ...messages.map(m => {
                const combinedContent = m.items
                    .filter(i => i.type === 'text')
                    .map(i => i.content)
                    .join('\n');
                return { 
                    role: m.role === 'user' ? 'user' : 'assistant', 
                    content: combinedContent 
                };
            }), 
            { role: 'user', content: currentInput } 
          ];
      }

      const response = await chatCompletion({
        fastgptAppId: id,
        chatId: targetChatId,
        stream: true,
        detail: false,
        shareId: shareId || '',
        outLinkUid: outLinkUid || '',
        messages: requestMessages
      });

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let aiContent = '';
      let aiReasoning = '';
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        
        if (value) {
            buffer += decoder.decode(value, { stream: true });
        }
        
        let shouldBreak = done;
        
        const lines = buffer.split('\n');
        // Keep the last part in the buffer as it might be incomplete if not done
        if (!shouldBreak) {
            buffer = lines.pop(); 
        } else {
            buffer = ''; // Process all if done
        }

        let chunkContent = '';
        let chunkReasoning = '';

        for (const line of lines) {
            const trimmedLine = line.trim();
            if (!trimmedLine.startsWith('data: ')) continue;

            const jsonStr = trimmedLine.slice(6).trim();
            if (jsonStr === '[DONE]') continue;
            
            try {
                let data = JSON.parse(jsonStr);
                
                // Handle double-encoded data property if present (FastGPT specific wrapper)
                if (data && data.data && typeof data.data === 'string') {
                        if (data.data === '[DONE]') continue;
                        try {
                            data = JSON.parse(data.data);
                        } catch (innerE) {
                            // console.warn('Failed to parse inner data JSON', innerE);
                        }
                }

                const content = data.choices?.[0]?.delta?.content || '';
                const reasoning = data.choices?.[0]?.delta?.reasoning_content || '';

                if (content) chunkContent += content;
                if (reasoning) chunkReasoning += reasoning;

            } catch (e) {
                console.error('Error parsing stream message', e);
            }
        }

        if (chunkContent || chunkReasoning) {
            aiContent += chunkContent;
            aiReasoning += chunkReasoning;
            
            setMessages(prev => prev.map(m => {
                if (m.key === aiMsgKey) {
                    const newItems = [];
                    if (aiReasoning) {
                        newItems.push({ type: 'reasoning', content: aiReasoning });
                    }
                    if (aiContent || (!aiReasoning && !aiContent)) {
                        newItems.push({ type: 'text', content: aiContent });
                    }
                    // If we have content but no reasoning ever, we already covered cases.
                    // If initial empty state, handled above.
                    
                    return { ...m, items: newItems };
                } 
                return m;
            }));
        }

        if (shouldBreak) break;
      }

      // 延迟刷新以确保后端数据一致性
      setTimeout(() => {
          loadMessages(targetChatId); // 刷新当前消息记录
          loadHistories(); // 刷新会话列表（标题等）
      }, 1000);

    } catch (err) {
      console.error(err);
      message.error('发送失败');
      setMessages(prev => prev.map(m => 
          m.role === 'ai' && m.content === '' ? { ...m, content: 'Error: Failed to get response' } : m
      ));
    } finally {
      setSending(false);
    }
  };

  const getGroupLabel = (time) => {
    const date = new Date(time);
    const now = new Date();
    if (date.toDateString() === now.toDateString()) return '今天';
    
    const yesterday = new Date(now);
    yesterday.setDate(now.getDate() - 1);
    if (date.toDateString() === yesterday.toDateString()) return '昨天';
    
    return '更早';
  };

  // 转换历史记录为 Conversations 需要的格式
  const conversationItems = histories.map(h => ({
    key: h.chatId,
    label: h.title || '新对话',
    icon: <EditOutlined />,
    group: getGroupLabel(h.updateTime || Date.now()),
  }));

  // 转换消息为 Bubble 需要的格式
  const bubbleItems = messages.flatMap((msg, msgIndex) => {
    // If loading
    if(msg.role === 'ai' && msg.items.length === 0 && !msg.content && sending) {
        return [{
            key: msg.key,
            placement: 'start',
            loading: true,
            avatar: <Avatar src={AI_AVATAR} icon={<RobotOutlined />} style={{ backgroundColor: token.colorFillSecondary }} />,
        }];
    }

    const bubbles = [];
    let pendingReasoningNode = null;
    
    msg.items.forEach((item, idx) => {
        let currentNode = null;

        // 1. Build the node for current item
        if (item.type === 'reasoning') {
            const isTyping = msg.role === 'ai' && 
                             sending && 
                             msgIndex === messages.length - 1 && 
                             idx === msg.items.length - 1;
            
             currentNode = (
                <ThinkingProcess 
                   content={item.content} 
                   isTyping={isTyping} 
                   duration={item.duration} 
                />
             );
        } else if (item.type === 'interactive') {
             currentNode = (
                <InteractiveOptions 
                   interactive={item.interactive}
                   onSelect={(val) => onSend(val, item.interactive)}
                />
             );
        } else if (item.type === 'text') {
             const isTyping = msg.role === 'ai' && 
                              sending && 
                              msgIndex === messages.length - 1 && 
                              idx === msg.items.length - 1;

             if (item.content || isTyping) {
                 currentNode = (
                    <TypingMarkdown 
                        content={item.content || ''} 
                        isTyping={isTyping} 
                        sourceList={msg.totalQuoteList}
                        onReferenceClick={(idx) => openReferenceDrawer(msg.totalQuoteList, idx)}
                    />
                 );
             }
        }

        // 2. Logic to merge reasoning with next item
        if (item.type === 'reasoning') {
            // If it's the last item, we must render it. 
            // Otherwise, store it and wait for next message item to merge.
            if (idx !== msg.items.length - 1) {
                pendingReasoningNode = currentNode;
                return; // Skip rendering this iteration, wait for next
            }
        }

        if (currentNode || (pendingReasoningNode && idx === msg.items.length - 1)) {
             const contentStack = [];
             
             // If we have a pending reasoning node, prepend it
             if (pendingReasoningNode) {
                 contentStack.push(
                     <div key={`reasoning-prev-${idx}`} style={{ marginBottom: 8 }}>
                         {pendingReasoningNode}
                     </div>
                 );
                 pendingReasoningNode = null;
             }
             
             if (currentNode) {
                 contentStack.push(currentNode);
             }

             const footerNodes = [];
             let footerElement = null;

             // Attach footer/quotes to the LAST item of the message group
             if (idx === msg.items.length - 1) {
                  if (msg.totalQuoteList && msg.totalQuoteList.length > 0) {
                      footerNodes.push(
                          <div key="quotes" style={{ marginTop: 8, paddingTop: 8, borderTop: '1px dashed #eee' }}>
                              <Typography.Text type="secondary" style={{ fontSize: 12, marginBottom: 4, display: 'block' }}>
                                  参考资料
                              </Typography.Text>
                              <ReferenceList 
                                  quotes={msg.totalQuoteList} 
                                  onOpen={(quotes, idx) => openReferenceDrawer(quotes, idx)}
                              />
                          </div>
                      );
                  }
                  
                  if (msg.durationSeconds || (msg.totalQuoteList && msg.totalQuoteList.length > 0)) {
                      footerElement = (
                          <div style={{ marginTop: 4, display: 'flex', gap: 16, fontSize: 12, color: '#999', justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start' }}>
                             {msg.totalQuoteList?.length > 0 && (
                                 <Space size={4}>
                                    <ReadOutlined />
                                    <span>{msg.totalQuoteList.length} 引用</span>
                                 </Space>
                             )}
                             {msg.durationSeconds && (
                                 <Space size={4}>
                                    <ClockCircleOutlined />
                                    <span>{msg.durationSeconds}s</span>
                                 </Space>
                             )}
                          </div>
                      );
                  }
             }

             bubbles.push({
                 key: `${msg.key}-${idx}`,
                 placement: msg.role === 'user' ? 'end' : 'start',
                 content: (
                     <div style={{ display: 'flex', flexDirection: 'column' }}>
                         {contentStack}
                         {footerNodes}
                     </div>
                 ),
                 footer: footerElement,
                 avatar: (
                    <Avatar 
                      src={msg.role === 'user' ? userInfo?.avatar : AI_AVATAR}
                      icon={msg.role === 'user' ? <UserOutlined /> : <RobotOutlined />} 
                      style={{ backgroundColor: msg.role === 'user' ? token.colorPrimary : token.colorFillSecondary }} 
                    />
                 ),
             });
        }
    });

    // Fallback for empty parsed items but existing content (legacy)
    if (bubbles.length === 0 && msg.content) {
          bubbles.push({
             key: msg.key,
             placement: msg.role === 'user' ? 'end' : 'start',
             content: <Typography.Text>{msg.content}</Typography.Text>,
             avatar: (
                <Avatar 
                  src={msg.role === 'user' ? userInfo?.avatar : AI_AVATAR}
                  icon={msg.role === 'user' ? <UserOutlined /> : <RobotOutlined />} 
                  style={{ backgroundColor: msg.role === 'user' ? token.colorPrimary : token.colorFillSecondary }} 
                />
             ),
          });
    }

    return bubbles;
  });

  const menuConfig = (item) => ({
    items: [
      {
        label: '删除',
        key: 'delete',
        icon: <DeleteOutlined />,
        danger: true,
        onClick: () => handleDeleteHistory(item.key),
      },
    ],
  });

  return (
    <XProvider theme={{ token: { colorPrimary: token.colorPrimary } }}>
      <div style={{ width: '100vw', height: '100vh', display: 'flex', overflow: 'hidden' }}>
        {/* Sidebar */}
        <div style={{ 
          width: 280, 
          borderRight: `1px solid ${token.colorBorderSecondary}`, 
          display: 'flex', 
          flexDirection: 'column',
          background: token.colorBgLayout
        }}>
          <div style={{ padding: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 16, cursor: 'pointer' }} onClick={() => navigate('/subjects')}>
                <LeftOutlined style={{ marginRight: 8 }} />
                <Typography.Title level={5} style={{ margin: 0 }}>返回</Typography.Title>
            </div>
            <div 
              onClick={handleNewChat}
              style={{
                background: token.colorBgContainer,
                border: `1px solid ${token.colorBorder}`,
                borderRadius: token.borderRadiusLG,
                padding: '12px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                cursor: 'pointer',
                transition: 'all 0.3s',
                marginBottom: 16,
                boxShadow: token.boxShadowTertiary
              }}
              onMouseEnter={(e) => e.currentTarget.style.borderColor = token.colorPrimary}
              onMouseLeave={(e) => e.currentTarget.style.borderColor = token.colorBorder}
            >
              <PlusOutlined style={{ color: token.colorPrimary, marginRight: 8 }} />
              <span style={{ fontWeight: 500 }}>开始新对话</span>
            </div>
          </div>
          
          <div style={{ flex: 1, overflowY: 'auto' }}>
            <Conversations
                items={conversationItems}
                activeKey={activeChatId}
                onActiveChange={setActiveChatId}
                menu={menuConfig}
                groupable
            />
          </div>
        </div>

        {/* Main Content */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', height: '100%', position: 'relative', background: token.colorBgContainer }}>
          {/* Header */}
          <div style={{ 
              padding: '16px 24px', 
              borderBottom: `1px solid ${token.colorBorderSecondary}`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between'
          }}>
             <Typography.Title level={4} style={{ margin: 0 }}>
               {title || 'AI 助手'}
             </Typography.Title>
             <Space>
               <ShareAltOutlined style={{ fontSize: 18, cursor: 'pointer', color: token.colorTextSecondary }} />
             </Space>
          </div>

          {/* Messages Area */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '24px' }}>
            <div style={{ maxWidth: 800, margin: '0 auto' }}>
                <Bubble.List items={bubbleItems} />
                <div ref={messagesEndRef} />
            </div>
          </div>

          {/* Input Area */}
          {!isPendingInteractive && (
              <div style={{ padding: '24px', paddingTop: 0 }}>
                 <div style={{ maxWidth: 800, margin: '0 auto' }}>
                     <Sender 
                        value={inputValue}
                        onChange={setInputValue}
                        onSubmit={onSend}
                        loading={sending}
                        placeholder="输入您的问题，Shift + Enter 换行"
                     />
                     <Typography.Text type="secondary" style={{ fontSize: 12, textAlign: 'center', display: 'block', marginTop: 8 }}>
                        内容由 AI 生成，请仔细甄别
                     </Typography.Text>
                 </div>
              </div>
          )}
        </div>
      </div>
      <ReferenceDrawer
          isOpen={referenceDrawerState.open}
          quotes={referenceDrawerState.quotes}
          startIndex={referenceDrawerState.index}
          onClose={() => setReferenceDrawerState(prev => ({ ...prev, open: false }))}
      />
    </XProvider>
  );
};

export default Chat;
