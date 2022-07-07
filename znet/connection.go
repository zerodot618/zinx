package znet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"zinx/utils"
	"zinx/ziface"
)

// IConnection 接口实现，定义一个 Connection 服务结构体
type Connection struct {
	// 当前Conn属于哪个Server
	// 当前conn属于哪个server，在conn初始化的时候添加即可
	TcpServer ziface.IServer
	// 当前连接的socker TCP 套接字
	Conn *net.TCPConn
	// 当前连接的ID，也可以称作为 SessionID，ID全局唯一
	ConnID uint32
	// 当前连接的关闭状态
	isClosed bool
	// 该连接的处理方法 router
	MsgHandler ziface.IMsgHandle
	// 告知该连接已经退出/停止的 channel
	ExitBuffChan chan bool
	// 无缓冲管道，用于读，写两个 goroutine 之间的消息通信
	msgChan chan []byte
	// 有缓冲管道，用于读，写两个 goroutine 之间的消息通信
	msgBuffChan chan []byte
}

// NewConnection 创建连接的方法
func NewConnection(server ziface.IServer, conn *net.TCPConn, connID uint32, msgHandler ziface.IMsgHandle) *Connection {
	// 初始化 Conn 属性
	c := &Connection{
		TcpServer:    server,
		Conn:         conn,
		ConnID:       connID,
		isClosed:     false,
		MsgHandler:   msgHandler,
		ExitBuffChan: make(chan bool, 1),
		msgChan:      make(chan []byte),
		msgBuffChan:  make(chan []byte, utils.GlobalObject.MaxMsgChanLen),
	}
	// 将新创建的Conn添加到连接管理中
	// 将当前新创建的连接添加到ConnManager中
	c.TcpServer.GetConnMgr().Add(c)
	return c
}

// StartWriter 写消息 Goroutine，用户将数据发送给客户端
func (c *Connection) StartWriter() {
	fmt.Println("[Writer Goroutine is running]")
	defer fmt.Println(c.RemoteAddr().String(), " [conn Writer exit!]")
	for {
		select {
		case data := <-c.msgChan:
			// 有数据要写给客户端
			if _, err := c.Conn.Write(data); err != nil {
				fmt.Println("Send Data error:, ", err, " Conn Writer exit")
				return
			}
		// 针对有缓冲channel需要些的数据处理
		case data, ok := <-c.msgBuffChan:
			if ok {
				// 有数据要写给客户端
				if _, err := c.Conn.Write(data); err != nil {
					fmt.Println("Send Data error:, ", err, " Conn Writer exit")
					return
				}
			} else {
				break
				// fmt.Println("msgBuffChan is Closed")
			}
		case <-c.ExitBuffChan:
			// conn 已经关闭
			return
		}
	}
}

// 处理 conn 读数据的 goroutine
func (c *Connection) StartReader() {
	fmt.Println("Reader Goroutine is running")
	defer fmt.Println(c.Conn.RemoteAddr().String(), " conn reader exit!")
	defer c.Stop()
	for {
		// 创建拆包解包的对象
		dp := NewDataPack()
		// 读取客户端的 Msg head
		headData := make([]byte, dp.GetHeadLen())
		if _, err := io.ReadFull(c.GetTCPConnection(), headData); err != nil {
			fmt.Println("read msg head error ", err)
			c.ExitBuffChan <- true
			continue
		}
		// 拆包，得到 msgId 和 dataLen 放在msg 中
		msg, err := dp.UnPack(headData)
		if err != nil {
			fmt.Println("unpack error ", err)
			c.ExitBuffChan <- true
			continue
		}
		// 根据 dataLen 读取 data，放在 msg.Data 中
		var data []byte
		if msg.GetDataLen() > 0 {
			data = make([]byte, msg.GetDataLen())
			if _, err := io.ReadFull(c.GetTCPConnection(), data); err != nil {
				fmt.Println("read msg data error ", err)
				c.ExitBuffChan <- true
				continue
			}
		}
		msg.SetData(data)
		// 得到当前客户端请求的 Request 数据
		req := Request{
			conn: c,
			msg:  msg,
		}
		if utils.GlobalObject.WorkerPoolSize > 0 {
			// 已经启动工作池机制，将消息交给worker处理
			c.MsgHandler.SendMsgToTaskQueue(&req)
		} else {
			// 从绑定好的消息和对应的处理方法中执行对应的Handle方法
			go c.MsgHandler.DoMsgHandler(&req)
		}
	}
}

// Start 启动连接，让当前连接开始工作
func (c *Connection) Start() {
	// 1. 开启用于从客户端都且数据流程的 Goroutine
	go c.StartReader()
	// 2. 开启用于回写客户端数据流程的 Goroutine
	go c.StartWriter()
	// 按照用户传递进来的创建连接时需要处理的业务，执行钩子方法
	c.TcpServer.CallOnConnStart(c)

	for {
		select {
		case <-c.ExitBuffChan:
			// 得到退出消息，不在阻塞
			return
		}
	}
}

// Stop 停止连接，结束当前连接状态 M
func (c *Connection) Stop() {
	fmt.Println("Conn Stop()...ConnID = ", c.ConnID)
	// 1. 如果当前连接已经关闭
	if c.isClosed {
		return
	}
	c.isClosed = true
	// 如果用户注册了该链接的关闭回调业务，那么在此刻应该显示调用
	c.TcpServer.CallOnConnStop(c)
	// 关闭 socket 连接
	c.Conn.Close()
	// 关闭 Writer Goroutine
	c.ExitBuffChan <- true
	// 将连接从连接管理器中删除, 从ConnManager中删除conn
	c.TcpServer.GetConnMgr().Remove(c)
	// 关闭该连接全部管道
	close(c.ExitBuffChan)
	close(c.msgBuffChan)
}

// GetTCPConnection 从当前连接获取原始的 socket TCPConn
func (c *Connection) GetTCPConnection() *net.TCPConn {
	return c.Conn
}

// 获取当前连接ID
func (c *Connection) GetConnID() uint32 {
	return c.ConnID
}

// 获取远程客户端地址信息
func (c *Connection) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

// SendMsg 直接将 Message数据发送数据给远程的TCP客户端
func (c *Connection) SendMsg(msgId uint32, data []byte) error {
	if c.isClosed {
		return errors.New("Connection closed when send msg")
	}
	// 将 data 封包，并且发送
	dp := NewDataPack()
	msg, err := dp.Pack(NewMsgPackage(msgId, data))
	if err != nil {
		fmt.Println("Pack error msg id = ", msgId)
		return errors.New("Pack error msg")
	}
	// 写回客户端
	c.msgChan <- msg //将之前直接回写给conn.Write的方法 改为 发送给Channel 供Writer读取
	return nil
}

// SendBuffMsg 直接将 Message数据发送数据给远程的TCP客户端(有缓冲)
func (c *Connection) SendBuffMsg(msgId uint32, data []byte) error {
	if c.isClosed {
		return errors.New("Connection closed when send buff msg")
	}
	// 将 data 封包，并且发送
	dp := NewDataPack()
	msg, err := dp.Pack(NewMsgPackage(msgId, data))
	if err != nil {
		fmt.Println("Pack error msg id = ", msgId)
		return errors.New("Pack error msg")
	}
	// 写回客户端
	c.msgBuffChan <- msg //将之前直接回写给conn.Write的方法 改为 发送给Channel 供Writer读取
	return nil
}
