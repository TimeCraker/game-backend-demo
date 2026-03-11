package utils

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
)

// 发送邮件的真实配置（请替换为你自己的真实配置）
const (
	SMTPHost     = "smtp.qq.com"       // QQ邮箱服务器为例
	SMTPPort     = "465"               // 一般是 465 或 587
	SenderEmail  = "2645068655@qq.com" // 你的发件邮箱
	SenderSecret = "wrwynnhosmzxeaja"  // 你的 SMTP 授权码
)

// SendVerificationEmail 通过 SMTP 发送具有 UI 排版的 HTML 验证码邮件
func SendVerificationEmail(toEmail string, code string) error {
	auth := smtp.PlainAuth("", SenderEmail, SenderSecret, SMTPHost)

	// ===== 邮件内容构建 START =====
	from := fmt.Sprintf("From: %s\r\n", SenderEmail)
	to := fmt.Sprintf("To: %s\r\n", toEmail)
	subject := "Subject: 游戏账号注册验证码\r\n"

	// 修改 Content-Type 为 text/html 确保 UI 正常显示
	contentType := "Content-Type: text/html; charset=UTF-8\r\n\r\n"

	// 使用 HTML 邮件模板，让验证码邮件具有 UI 排版
	htmlBody := fmt.Sprintf(`
<html>
<body style="margin:0;padding:0;background:#f4f6f8;font-family:Arial,Helvetica,sans-serif;">
	<div style="width:100%%;padding:40px 0;">
		<div style="
			max-width:500px;
			margin:auto;
			background:white;
			border-radius:10px;
			padding:30px;
			box-shadow:0 4px 12px rgba(0,0,0,0.1);
			text-align:center;
		">
			<h2 style="color:#333;">欢迎注册 TimeCraker 的游戏！</h2>

			<p style="font-size:16px;color:#666;margin-top:20px;">
				您的验证码是
			</p>

			<div style="
				font-size:36px;
				font-weight:bold;
				letter-spacing:6px;
				color:#2d8cf0;
				margin:20px 0;
			">
				%s
			</div>

			<p style="font-size:14px;color:#999;">
				该验证码 <b>5 分钟内有效</b>，请勿将验证码泄露给他人。
			</p>

			<hr style="margin:30px 0;border:none;border-top:1px solid #eee;">

			<p style="font-size:12px;color:#aaa;">
				TimeCraker Game System
			</p>
		</div>
	</div>
</body>
</html>
`, code)

	msg := []byte(from + to + subject + contentType + htmlBody)
	// ===== 邮件内容构建 END =====

	addr := fmt.Sprintf("%s:%s", SMTPHost, SMTPPort)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		log.Printf("❌ SMTP TLS 连接失败: %v", err)
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, SMTPHost)
	if err != nil {
		return err
	}

	if err = client.Auth(auth); err != nil {
		log.Printf("❌ SMTP 身份认证失败: %v", err)
		return err
	}

	// 1. 设置发件人
	if err = client.Mail(SenderEmail); err != nil {
		return err
	}
	// 2. 设置收件人
	if err = client.Rcpt(toEmail); err != nil {
		return err
	}
	// 3. 写入数据流
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	log.Printf("✅ HTML 格式验证码邮件已成功发送至: %s", toEmail)
	return client.Quit()
}
