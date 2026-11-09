package service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"time"
)

// SendInvoicePDF uses the configured SMTP transport with a binary PDF attachment.
func (s *EmailService) SendInvoicePDF(to, orderNo, name string, data []byte) error {
	from, addr, err := s.prepareSMTPEnvelope(to)
	if err != nil {
		return err
	}
	msg, err := buildInvoiceEmail(from, to, orderNo, name, data)
	if err != nil {
		return err
	}
	return s.sendInvoiceSMTPMessage(addr, to, msg)
}
func buildInvoiceEmail(from, to, orderNo, name string, data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writeStandardHeaders(&buf, from, to, "订单电子发票")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", writer.Boundary())
	body, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}, "Content-Transfer-Encoding": {"base64"}})
	if err != nil {
		return nil, err
	}
	writeEncoded := func(data []byte) []byte {
		encoded := base64.StdEncoding.EncodeToString(data)
		var out bytes.Buffer
		for len(encoded) > 76 {
			out.WriteString(encoded[:76] + "\r\n")
			encoded = encoded[76:]
		}
		out.WriteString(encoded + "\r\n")
		return out.Bytes()
	}
	if _, err = body.Write(writeEncoded([]byte("您好，订单 " + orderNo + " 的电子发票已开具，请查收附件。如有疑问，请联系商家。"))); err != nil {
		return nil, err
	}
	attachment, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/pdf"}, "Content-Disposition": {mime.FormatMediaType("attachment", map[string]string{"filename": name})}, "Content-Transfer-Encoding": {"base64"}})
	if err != nil {
		return nil, err
	}
	if _, err = attachment.Write(writeEncoded(data)); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Bound the entire SMTP exchange to less than the HTTP client timeout and lease.
func (s *EmailService) sendInvoiceSMTPMessage(addr, to string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
		return err
	}
	var transport net.Conn = conn
	tlsConfig := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
	if s.cfg.UseSSL {
		transport = tls.Client(conn, tlsConfig)
	}
	client, err := smtp.NewClient(transport, s.cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if !s.cfg.UseSSL && s.cfg.UseTLS {
		if err = client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if err = authenticateSMTPClient(client, s.cfg.Host, s.cfg.Username, s.cfg.Password); err != nil {
		return err
	}
	return normalizeEmailSendError(sendSMTPData(client, s.cfg.Host, addr, s.cfg.From, []string{to}, msg))
}
