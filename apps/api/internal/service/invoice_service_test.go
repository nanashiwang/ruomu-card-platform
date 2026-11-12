package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/queue"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var invoiceTestPDF = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF\n")

func invoiceFixture(t *testing.T) (*InvoiceService, models.Order) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:invoice-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&models.Order{}, &models.OrderInvoice{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	order := models.Order{OrderNo: "TEST-1", UserID: 17, Status: "paid", Currency: "CNY", PaidAt: &now, TotalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)), RefundedAmount: models.NewMoneyFromDecimal(decimal.Zero)}
	if err = db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	return &InvoiceService{db: db, send: func(string, string, string, []byte) error { return nil }}, order
}
func invoiceInput() InvoiceInput {
	return InvoiceInput{Kind: "company", Title: "测试公司", TaxID: "91310000123456789X", Email: "invoice@example.test"}
}
func issueInvoice(t *testing.T, s *InvoiceService, o models.Order) *models.OrderInvoice {
	t.Helper()
	item, err := s.Apply(o.ID, o.UserID, invoiceInput())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Review(item.ID, 8, true, ""); err != nil {
		t.Fatal(err)
	}
	if err = s.Upload(item.ID, 8, invoiceTestPDF); err != nil {
		t.Fatal(err)
	}
	return item
}
func TestInvoiceLifecycleAndMailRetry(t *testing.T) {
	s, o := invoiceFixture(t)
	item, err := s.Apply(o.ID, o.UserID, invoiceInput())
	if err != nil {
		t.Fatal(err)
	}
	if !item.Amount.Equal(decimal.NewFromInt(100)) || item.Currency != "CNY" {
		t.Fatal(item)
	}
	if _, err = s.Apply(o.ID, o.UserID, invoiceInput()); !errors.Is(err, ErrInvoiceState) {
		t.Fatal("duplicate", err)
	}
	if err = s.Upload(item.ID, 8, invoiceTestPDF); !errors.Is(err, ErrInvoiceState) {
		t.Fatal("unreviewed upload", err)
	}
	if err = s.Review(item.ID, 8, false, ""); !errors.Is(err, ErrInvoiceInvalid) {
		t.Fatal("empty rejection", err)
	}
	if err = s.Review(item.ID, 8, false, "请更正抬头"); err != nil {
		t.Fatal(err)
	}
	in := invoiceInput()
	in.Title = "正确公司"
	again, err := s.Apply(o.ID, o.UserID, in)
	if err != nil || again.ID != item.ID || again.RejectionReason != "" {
		t.Fatal(again, err)
	}
	if err = s.Review(item.ID, 8, true, ""); err != nil {
		t.Fatal(err)
	}
	if err = s.Review(item.ID, 8, false, "重复"); !errors.Is(err, ErrInvoiceState) {
		t.Fatal(err)
	}
	if err = s.Upload(item.ID, 8, invoiceTestPDF); err != nil {
		t.Fatal(err)
	}
	if err = s.Upload(item.ID, 8, invoiceTestPDF); !errors.Is(err, ErrInvoiceState) {
		t.Fatal("duplicate upload", err)
	}
	attempts := 0
	s.send = func(to, order, name string, data []byte) error {
		attempts++
		if to != in.Email || order != o.OrderNo || !bytes.Equal(data, invoiceTestPDF) {
			t.Error("attachment/recipient mismatch")
		}
		return errors.New("SMTP secret internal detail")
	}
	if err = s.Send(item.ID); !errors.Is(err, ErrInvoiceMail) {
		t.Fatal(err)
	}
	failed, _ := s.ForOrder(o.ID)
	if failed.Status != "issued" || failed.MailStatus != "failed" || strings.Contains(failed.MailError, "secret") {
		t.Fatal(failed)
	}
	s.send = func(string, string, string, []byte) error { attempts++; return nil }
	if err = s.Send(item.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Send(item.ID); err != nil {
		t.Fatal(err)
	}
	done, _ := s.ForOrder(o.ID)
	if attempts != 2 || done.MailStatus != "sent" || done.SentAt == nil {
		t.Fatal(attempts, done)
	}
}
func TestInvoiceRejectInvalidOrders(t *testing.T) {
	for _, scenario := range []string{"unpaid", "canceled", "refunded", "partial", "child", "guest", "zero", "other_user", "child_refund"} {
		t.Run(scenario, func(t *testing.T) {
			s, o := invoiceFixture(t)
			user := o.UserID
			switch scenario {
			case "unpaid":
				o.PaidAt = nil
			case "canceled":
				o.Status = "canceled"
			case "refunded":
				o.Status = "refunded"
			case "partial":
				o.RefundedAmount = models.NewMoneyFromDecimal(decimal.NewFromInt(1))
			case "child":
				id := uint(99)
				o.ParentID = &id
			case "guest":
				o.UserID = 0
			case "zero":
				o.TotalAmount = models.NewMoneyFromDecimal(decimal.Zero)
			case "other_user":
				user = 88
			case "child_refund":
				child := models.Order{OrderNo: "CHILD", ParentID: &o.ID, UserID: o.UserID, Status: "refunded", Currency: "CNY"}
				if err := s.db.Create(&child).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := s.db.Save(&o).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.Apply(o.ID, user, invoiceInput()); err == nil {
				t.Fatal("invalid application allowed")
			}
		})
	}
}
func TestInvoiceValidation(t *testing.T) {
	for _, change := range []func(*InvoiceInput){func(i *InvoiceInput) { i.Email = "name <a@example.test>" }, func(i *InvoiceInput) { i.Email = "a@example.test\r\nBcc: x@example.test" }, func(i *InvoiceInput) { i.Email = "telegram_123@login.local" }, func(i *InvoiceInput) { i.Title = " " }, func(i *InvoiceInput) { i.TaxID = "BAD" }, func(i *InvoiceInput) { i.Kind = "bad" }, func(i *InvoiceInput) { i.Note = strings.Repeat("字", 1001) }} {
		s, o := invoiceFixture(t)
		in := invoiceInput()
		change(&in)
		if _, err := s.Apply(o.ID, o.UserID, in); !errors.Is(err, ErrInvoiceInvalid) {
			t.Fatal(in, err)
		}
	}
	s, o := invoiceFixture(t)
	in := invoiceInput()
	in.Kind = "personal"
	item, err := s.Apply(o.ID, o.UserID, in)
	if err != nil || item.TaxID != "" {
		t.Fatal(item, err)
	}
}
func TestInvoiceRechecksRefundBeforeReviewUploadSend(t *testing.T) {
	for _, step := range []string{"review", "upload", "send"} {
		t.Run(step, func(t *testing.T) {
			s, o := invoiceFixture(t)
			item, err := s.Apply(o.ID, o.UserID, invoiceInput())
			if err != nil {
				t.Fatal(err)
			}
			if step != "review" {
				if err = s.Review(item.ID, 8, true, ""); err != nil {
					t.Fatal(err)
				}
			}
			if step == "send" {
				if err = s.Upload(item.ID, 8, invoiceTestPDF); err != nil {
					t.Fatal(err)
				}
			}
			if err = s.db.Model(&models.Order{}).Where("id = ?", o.ID).Update("refunded_amount", 1).Error; err != nil {
				t.Fatal(err)
			}
			switch step {
			case "review":
				err = s.Review(item.ID, 8, true, "")
			case "upload":
				err = s.Upload(item.ID, 8, invoiceTestPDF)
			case "send":
				err = s.Send(item.ID)
			}
			if !errors.Is(err, ErrInvoiceState) {
				t.Fatal(step, err)
			}
		})
	}
}
func TestInvoiceDocumentPrivacyAndValidation(t *testing.T) {
	s, o := invoiceFixture(t)
	item := issueInvoice(t, s, o)
	for _, data := range [][]byte{[]byte("not a pdf"), []byte("%PDF-no eof"), bytes.Repeat([]byte("x"), InvoicePDFLimit+1)} {
		if err := s.Upload(item.ID, 8, data); !errors.Is(err, ErrInvoicePDF) {
			t.Fatal(err)
		}
	}
	doc, err := s.Document(item.ID)
	if err != nil || !bytes.Equal(doc.FileData, invoiceTestPDF) {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(doc)
	if bytes.Contains(encoded, []byte("file_data")) || bytes.Contains(encoded, []byte(base64.StdEncoding.EncodeToString(invoiceTestPDF))) {
		t.Fatal("document leaked through JSON")
	}
	list, total, err := s.List(1, "issued", o.OrderNo)
	if err != nil || total != 1 || len(list) != 1 || len(list[0].FileData) != 0 {
		t.Fatal(list, total, err)
	}
}
func TestInvoiceConcurrentSendAndStaleLease(t *testing.T) {
	s, o := invoiceFixture(t)
	item := issueInvoice(t, s, o)
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	s.send = func(string, string, string, []byte) error { close(entered); <-release; return nil }
	go func() { done <- s.Send(item.ID) }()
	<-entered
	err := s.Send(item.ID)
	close(release)
	first := <-done
	if !errors.Is(err, ErrInvoiceState) || first != nil {
		t.Fatal(err, first)
	}
	old := time.Now().UTC().Add(-11 * time.Minute)
	if err = s.db.Model(&models.OrderInvoice{}).Where("id = ?", item.ID).Updates(map[string]any{"mail_status": "sending", "sending_at": old}).Error; err != nil {
		t.Fatal(err)
	}
	s.send = func(string, string, string, []byte) error { return nil }
	if err = s.Send(item.ID); err != nil {
		t.Fatal("stale lease", err)
	}
}
func TestInvoiceEmailBinaryMIME(t *testing.T) {
	pdf := append(append([]byte{}, invoiceTestPDF...), []byte{0, 255, 128, 13, 10}...)
	raw, err := buildInvoiceEmail("shop@example.test", "a@example.test", "ORDER-1", "invoice-1.pdf", pdf)
	if err != nil {
		t.Fatal(err)
	}
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	media, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	if err != nil || media != "multipart/mixed" {
		t.Fatal(media, err)
	}
	reader := multipart.NewReader(message.Body, params["boundary"])
	body, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	attachment, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if attachment.Header.Get("Content-Type") != "application/pdf" || attachment.FileName() != "invoice-1.pdf" {
		t.Fatal(attachment.Header)
	}
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, attachment))
	if err != nil || !bytes.Equal(decoded, pdf) {
		t.Fatal("binary attachment corrupted", err)
	}
	email := NewEmailService(&config.EmailConfig{})
	if err = email.SendInvoicePDF("a@example.test", "1", "invoice.pdf", pdf); !errors.Is(err, ErrEmailServiceDisabled) {
		t.Fatal(err)
	}
}

func TestInvoiceAmountSnapshotGuard(t *testing.T) {
	s, o := invoiceFixture(t)
	item, err := s.Apply(o.ID, o.UserID, invoiceInput())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.db.Model(&models.Order{}).Where("id = ?", o.ID).Update("total_amount", 99).Error; err != nil {
		t.Fatal(err)
	}
	if err = s.Review(item.ID, 8, true, ""); !errors.Is(err, ErrInvoiceState) {
		t.Fatal("changed amount approved", err)
	}
}

func TestInvoiceSMTPTransport(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			serverErr <- e
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		protocol := textproto.NewConn(conn)
		protocol.PrintfLine("220 localhost SMTP")
		for {
			line, e := protocol.ReadLine()
			if e != nil {
				serverErr <- e
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"), strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				protocol.PrintfLine("250 OK")
			case line == "DATA":
				protocol.PrintfLine("354 Send message")
				data, e := protocol.ReadDotBytes()
				if e != nil {
					serverErr <- e
					return
				}
				received <- data
				protocol.PrintfLine("250 accepted")
			case line == "QUIT":
				protocol.PrintfLine("221 Bye")
				serverErr <- nil
				return
			default:
				serverErr <- fmt.Errorf("unexpected SMTP command %q", line)
				return
			}
		}
	}()
	s, o := invoiceFixture(t)
	item := issueInvoice(t, s, o)
	email := NewEmailService(&config.EmailConfig{Enabled: true, Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, From: "shop@example.test"})
	s.send = email.SendInvoicePDF
	if err = s.Send(item.ID); err != nil {
		t.Fatal(err)
	}
	if err = <-serverErr; err != nil {
		t.Fatal(err)
	}
	raw := <-received
	if !bytes.Contains(raw, []byte("application/pdf")) || !bytes.Contains(raw, []byte("To: invoice@example.test")) {
		t.Fatal("SMTP envelope or PDF missing")
	}
	stored, _ := s.ForOrder(o.ID)
	if stored.MailStatus != "sent" || stored.SentAt == nil {
		t.Fatal(stored)
	}
}

func TestInvoiceConcurrentApplicationsAndReviews(t *testing.T) {
	s, o := invoiceFixture(t)
	for _, action := range []string{"apply", "review"} {
		done := make(chan error, 6)
		var item *models.OrderInvoice
		if action == "review" {
			item, _ = s.ForOrder(o.ID)
		}
		for i := 0; i < 6; i++ {
			go func() {
				if action == "apply" {
					_, err := s.Apply(o.ID, o.UserID, invoiceInput())
					done <- err
				} else {
					done <- s.Review(item.ID, 8, true, "")
				}
			}()
		}
		successes := 0
		for i := 0; i < 6; i++ {
			if err := <-done; err == nil {
				successes++
			} else if !errors.Is(err, ErrInvoiceState) {
				t.Fatal(err)
			}
		}
		if successes != 1 {
			t.Fatalf("%s succeeded %d times", action, successes)
		}
	}
	var count int64
	s.db.Model(&models.OrderInvoice{}).Count(&count)
	if count != 1 {
		t.Fatal(count)
	}
}

func TestInvoiceApplicationNotification(t *testing.T) {
	s, o := invoiceFixture(t)
	calls := 0
	s.notify = func(input NotificationEnqueueInput) error {
		calls++
		// The application must already be committed when notifications are enqueued.
		item, err := s.ForOrder(o.ID)
		if err != nil || item == nil || item.Status != "pending" {
			t.Fatalf("notification before commit: %v", err)
		}
		if input.EventType != "invoice_requested" || input.BizID != o.ID || input.Data["order_no"] != o.OrderNo || !input.Force {
			t.Fatalf("unexpected payload: %+v", input)
		}
		if _, ok := input.Data["tax_id"]; ok {
			t.Fatal("tax ID leaked into notification")
		}
		return errors.New("queue unavailable")
	}
	if _, err := s.Apply(o.ID, o.UserID+1, invoiceInput()); err == nil {
		t.Fatal("wrong owner accepted")
	}
	if calls != 0 {
		t.Fatal("invalid application notified")
	}
	item, err := s.Apply(o.ID, o.UserID, invoiceInput())
	if err != nil || item == nil || calls != 1 {
		t.Fatalf("queue failure affected application: %v, calls=%d", err, calls)
	}
	if _, err = s.Apply(o.ID, o.UserID, invoiceInput()); err == nil {
		t.Fatal("duplicate accepted")
	}
	if calls != 1 {
		t.Fatal("duplicate notified")
	}
	if err = s.Review(item.ID, 8, false, "请修改抬头"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(o.ID, o.UserID, invoiceInput()); err != nil || calls != 2 {
		t.Fatalf("resubmission notification: %v, calls=%d", err, calls)
	}
}

func TestInvoiceNotificationSettingCompatibility(t *testing.T) {
	defaults := NotificationCenterDefaultSetting()
	if defaults.Scenes.IsSceneEnabled("invoice_requested") {
		t.Fatal("new scene must be opt-in")
	}
	defaults.Scenes.InvoiceRequested = true
	restored := notificationCenterSettingFromJSON(NotificationCenterSettingToMap(defaults), NotificationCenterDefaultSetting())
	if !restored.Scenes.IsSceneEnabled("invoice_requested") || !isNotificationEventSupported("invoice_requested") {
		t.Fatal("scene lost on round trip")
	}
	for _, locale := range []string{"zh-CN", "zh-TW", "en-US"} {
		tmpl := restored.Templates.TemplateByEvent("invoice_requested").ResolveLocaleTemplate(locale)
		body := renderNotificationTemplate(tmpl.Body, buildNotificationTestVariables("invoice_requested", locale))
		if tmpl.Title == "" || !strings.Contains(body, "TEST-INVOICE-001") || strings.Contains(body, "{{") {
			t.Fatalf("invalid %s template: %s", locale, body)
		}
	}
}

func TestInvoiceNotificationToggleAndRetry(t *testing.T) {
	settings := NewSettingService(newMockSettingRepo())
	enabled := true
	recipient := []string{"ops@example.test"}
	title := "发票待办"
	_, err := settings.PatchNotificationCenterSetting(NotificationCenterSettingPatch{
		Channels:  &NotificationChannelsPatch{Email: &NotificationChannelPatch{Enabled: &enabled, Recipients: &recipient}},
		Templates: &NotificationTemplatesPatch{InvoiceRequested: &NotificationSceneTemplatePatch{ZHCN: &NotificationLocalizedTemplatePatch{Title: &title}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sender := &NotificationService{settingService: settings}
	payload := queue.NotificationDispatchPayload{EventType: "invoice_requested", Force: true}
	if err := sender.Dispatch(context.Background(), payload); err != nil {
		t.Fatalf("disabled scene sent: %v", err)
	}
	_, err = settings.PatchNotificationCenterSetting(NotificationCenterSettingPatch{Scenes: &NotificationScenePatch{InvoiceRequested: &enabled}})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := settings.GetNotificationCenterSetting()
	if err != nil || saved.Templates.InvoiceRequested.ZHCN.Title != title || !saved.Scenes.ExceptionAlert {
		t.Fatalf("patch lost adjacent settings: %v", err)
	}
	// Delivery failures must remain retryable instead of being swallowed by time dedupe.
	for i := 0; i < 2; i++ {
		if err := sender.Dispatch(context.Background(), payload); !errors.Is(err, ErrNotificationSendFailed) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	enabled = false
	if _, err = settings.PatchNotificationCenterSetting(NotificationCenterSettingPatch{Scenes: &NotificationScenePatch{InvoiceRequested: &enabled}}); err != nil {
		t.Fatal(err)
	}
	if err := sender.Dispatch(context.Background(), payload); err != nil {
		t.Fatalf("disabled scene retried: %v", err)
	}
}
