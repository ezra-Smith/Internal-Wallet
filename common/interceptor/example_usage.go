package interceptor

// 本文件展示如何在后端服务中使用元数据拦截器

/*
===========================================
1. 在后端 gRPC 服务端配置拦截器
===========================================

在你的 gRPC 服务启动时，添加服务端拦截器：

```go
package main

import (
    "internalwallet/common/interceptor"
    "github.com/zeromicro/go-zero/zrpc"
)

func main() {
    // 创建 gRPC 服务器配置
    c := zrpc.RpcServerConf{
        // ... 其他配置
    }

    // 启动服务器，添加元数据拦截器
    server := zrpc.MustNewServer(c, func(grpcServer *grpc.Server) {
        // 注册你的服务
        pb.RegisterYourServiceServer(grpcServer, &YourServiceImpl{})
    })

    // 添加服务端拦截器（自动提取客户端元数据）
    server.AddUnaryInterceptors(interceptor.ServerMetadataUnaryInterceptor())

    server.Start()
}
```

或者在 go-zero 的配置中使用：

```go
import (
    "internalwallet/common/interceptor"
    "github.com/zeromicro/go-zero/core/conf"
    "github.com/zeromicro/go-zero/zrpc"
)

func main() {
    var c config.Config
    conf.MustLoad(*configFile, &c)

    // 创建 RPC 服务器
    server := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
        pb.RegisterAccountServer(grpcServer, logic.NewAccountServer(c))
    })

    // 添加元数据拦截器
    server.AddUnaryInterceptors(interceptor.ServerMetadataUnaryInterceptor())

    server.Start()
}
```

===========================================
2. 在业务逻辑中使用元数据
===========================================

在你的 gRPC 方法实现中，可以从 Context 提取元数据：

```go
package logic

import (
    "context"
    "internalwallet/common/middleware"
    "internalwallet/proto/pb"
)

type RegisterLogic struct {
    // ...
}

func (l *RegisterLogic) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
    // 方式1：单独获取各个字段
    clientIP := middleware.GetClientIP(ctx)
    userAgent := middleware.GetUserAgent(ctx)
    deviceFingerprint := middleware.GetDeviceFingerprint(ctx)
    deviceID := middleware.GetDeviceID(ctx)
    platform := middleware.GetPlatform(ctx)
    appVersion := middleware.GetAppVersion(ctx)

    // 方式2：一次性获取所有元数据
    metadata := middleware.GetClientMetadata(ctx)

    // 使用元数据进行业务逻辑处理
    // 例如：记录注册IP
    user := &User{
        Email:      req.Email,
        RegisterIP: clientIP,  // 来自 HTTP 请求
        RegisterDevice: deviceID,
    }

    // 风控检查：同IP注册限制
    count, err := l.svcCtx.UserModel.CountByIPToday(ctx, clientIP)
    if err != nil {
        return nil, err
    }
    if count >= 3 {
        return nil, errors.New("该IP今日注册次数已达上限")
    }

    // 记录登录日志
    loginLog := &LoginLog{
        UserID:      user.ID,
        IPAddress:   metadata.IP,
        DeviceID:    metadata.DeviceID,
        DeviceType:  extractDeviceType(metadata.UserAgent),
        Platform:    metadata.Platform,
        AppVersion:  metadata.AppVersion,
        UserAgent:   metadata.UserAgent,
    }

    return &pb.RegisterResp{
        Success: true,
    }, nil
}
```

===========================================
3. 在 Model 层使用元数据
===========================================

如果需要在 Model 层记录IP等信息，可以通过参数传递：

```go
package model

type UserModel struct {
    // ...
}

// CreateWithMetadata 创建用户（带元数据）
func (m *UserModel) CreateWithMetadata(ctx context.Context, user *User, ip, deviceID string) error {
    user.RegisterIP = ip
    user.RegisterDevice = deviceID
    user.LastLoginIP = ip
    user.LastLoginTime = time.Now()

    _, err := m.Insert(ctx, user)
    return err
}
```

在 Logic 层调用：

```go
clientIP := middleware.GetClientIP(ctx)
deviceID := middleware.GetDeviceID(ctx)

err := l.svcCtx.UserModel.CreateWithMetadata(ctx, user, clientIP, deviceID)
```

===========================================
4. 实际应用场景
===========================================

4.1 注册风控
```go
func (l *RegisterLogic) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
    clientIP := middleware.GetClientIP(ctx)

    // 检查同IP注册次数
    count, _ := l.countRegistrationByIP(ctx, clientIP)
    if count >= 3 {
        return nil, errors.New("该IP今日注册次数已达上限")
    }

    // 检测虚拟运营商号段
    if isVirtualOperator(req.Phone) {
        return nil, errors.New("不支持虚拟运营商号码")
    }

    // 检测临时邮箱
    if isTempEmail(req.Email) {
        return nil, errors.New("不支持临时邮箱")
    }

    // ... 创建用户
}
```

4.2 登录异常检测
```go
func (l *LoginLogic) Login(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
    metadata := middleware.GetClientMetadata(ctx)

    // 查询用户最后登录IP
    user, err := l.svcCtx.UserModel.FindByEmail(ctx, req.Email)
    if err != nil {
        return nil, err
    }

    // 异地登录检测
    if user.LastLoginIP != "" && user.LastLoginIP != metadata.IP {
        // 触发异地登录通知
        l.sendAbnormalLoginNotification(ctx, user, metadata)
    }

    // 记录登录日志
    loginLog := &LoginLog{
        UserID:      user.ID,
        IPAddress:   metadata.IP,
        IPLocation:  l.getIPLocation(metadata.IP),
        DeviceID:    metadata.DeviceID,
        UserAgent:   metadata.UserAgent,
        IsAbnormal:  isAbnormalLogin(user, metadata),
    }
    l.svcCtx.LoginLogModel.Insert(ctx, loginLog)

    // ... 生成token等
}
```

4.3 设备管理
```go
func (l *DeviceLogic) RecordDevice(ctx context.Context, userID int64) error {
    metadata := middleware.GetClientMetadata(ctx)

    device := &UserDevice{
        UserID:         userID,
        DeviceID:       metadata.DeviceID,
        DeviceName:     extractDeviceName(metadata.UserAgent),
        DeviceType:     metadata.Platform,
        LastLoginIP:    metadata.IP,
        LastLoginTime:  time.Now(),
    }

    return l.svcCtx.DeviceModel.Upsert(ctx, device)
}
```

4.4 KYC人脸识别
```go
func (l *KYCLogic) FaceVerify(ctx context.Context, req *pb.FaceVerifyReq) (*pb.FaceVerifyResp, error) {
    clientIP := middleware.GetClientIP(ctx)

    // 更新失败次数和锁定状态
    verification := &KYCVerification{
        UserID:                userID,
        FaceVerifyFailCount:   failCount + 1,
    }

    // 失败3次锁定
    if failCount+1 >= 3 {
        verification.FaceVerifyLockUntil = time.Now().Add(24 * time.Hour)
        return nil, errors.New("人脸验证失败次数过多，请24小时后重试")
    }

    // 记录验证IP
    // ...
}
```

===========================================
5. 前端需要传递的Header
===========================================

前端在发送HTTP请求时，需要携带以下自定义Header：

```javascript
// Web 端
fetch('/api/v1/auth/register', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Device-Fingerprint': generateFingerprint(),  // 浏览器指纹
    'X-Device-ID': getDeviceID(),                   // 设备ID（存储在localStorage）
    'X-Platform': 'web',                            // 平台标识
    'X-App-Version': '1.0.0',                       // 版本号
  },
  body: JSON.stringify(data)
});

// iOS/Android
headers: {
  'X-Device-Fingerprint': deviceFingerprint,
  'X-Device-ID': deviceUUID,
  'X-Platform': 'ios',  // or 'android'
  'X-App-Version': '1.2.3',
}
```

浏览器指纹生成示例（使用 FingerprintJS）：
```javascript
import FingerprintJS from '@fingerprintjs/fingerprintjs';

async function generateFingerprint() {
  const fp = await FingerprintJS.load();
  const result = await fp.get();
  return result.visitorId;
}
```

===========================================
6. 注意事项
===========================================

1. IP获取优先级：X-Forwarded-For > X-Real-IP > RemoteAddr
2. 在Nginx等反向代理后，需要正确配置 X-Forwarded-For
3. 设备指纹应该在客户端生成，不要后端生成
4. 敏感操作（提现、修改密码等）建议额外验证IP和设备
5. 定期清理过期的登录日志和设备记录

===========================================
7. Nginx 配置示例
===========================================

```nginx
location /api/ {
    proxy_pass http://gateway:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

*/
