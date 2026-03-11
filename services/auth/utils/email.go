package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

// 发送邮件的真实配置（请替换为你自己的真实配置）
const (
	SMTPHost     = "smtp.qq.com"       // QQ邮箱服务器为例
	SMTPPort     = "465"               // 一般是 465 或 587
	SenderEmail  = "2645068655@qq.com" // 你的发件邮箱
	SenderSecret = "wrwynnhosmzxeaja"  // 你的 SMTP 授权码
)

// SendVerificationEmail 通过 SMTP 发送真实的验证码邮件
func SendVerificationEmail(toEmail string, code string) error {
	auth := smtp.PlainAuth("", SenderEmail, SenderSecret, SMTPHost)

	// 构造邮件内容，包含基本的 HTML 和发件人头部
	subject := "Subject: 游戏账号注册验证码\n"
	contentType := "Content-Type: text/plain; charset=UTF-8\n\n"
	body := fmt.Sprintf("欢迎注册游戏！您的验证码是：%s。该验证码在5分钟内有效。请勿将验证码泄露给他人。", code)
	msg := []byte(subject + contentType + body)

	addr := fmt.Sprintf("%s:%s", SMTPHost, SMTPPort)

	// 大多数国内邮箱强制使用 TLS 发送
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, SMTPHost)
	if err != nil {
		return err
	}

	if err = client.Auth(auth); err != nil {
		return err
	}
	if err = client.Mail(SenderEmail); err != nil {
		return err
	}
	if err = client.Rcpt(toEmail); err != nil {
		return err
	}

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

	return client.Quit()
}
