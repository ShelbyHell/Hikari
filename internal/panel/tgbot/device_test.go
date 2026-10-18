package tgbot

import (
	"fmt"
	"strings"
	"testing"

	"mikan/internal/panel/billing"
)

func TestDeviceWords(t *testing.T) {
	msgRu := fmt.Sprintf(ru.paidDevice, "Тест")
	if !strings.Contains(msgRu, "Тест") || !strings.Contains(msgRu, "увеличен на 1") {
		t.Errorf("ru.paidDevice unexpected: %q", msgRu)
	}

	refRu := fmt.Sprintf(ru.refundedDevice, "Тест")
	if !strings.Contains(refRu, "Тест") || !strings.Contains(refRu, "уменьшен") {
		t.Errorf("ru.refundedDevice unexpected: %q", refRu)
	}

	msgEn := fmt.Sprintf(en.paidDevice, "Test")
	if !strings.Contains(msgEn, "Test") || !strings.Contains(msgEn, "increased by 1") {
		t.Errorf("en.paidDevice unexpected: %q", msgEn)
	}

	refEn := fmt.Sprintf(en.refundedDevice, "Test")
	if !strings.Contains(refEn, "Test") || !strings.Contains(refEn, "decreased") {
		t.Errorf("en.refundedDevice unexpected: %q", refEn)
	}
}

func TestDevicePayButtons(t *testing.T) {
	av := billing.Available{Stars: true, Addons: []string{"yookassa", "cryptobot"}}
	buttons := ru.payButtons("da:", 50, 10000, av, func(id string) string { return id })
	if len(buttons) != 3 {
		t.Fatalf("expected 3 payment buttons, got %d", len(buttons))
	}
	if buttons[0][0].CallbackData != "da:s" {
		t.Errorf("expected stars callback da:s, got %s", buttons[0][0].CallbackData)
	}
	if buttons[1][0].CallbackData != "da:y" {
		t.Errorf("expected yookassa callback da:y, got %s", buttons[1][0].CallbackData)
	}
	if buttons[2][0].CallbackData != "da:c" {
		t.Errorf("expected cryptobot callback da:c, got %s", buttons[2][0].CallbackData)
	}

	for _, row := range buttons {
		btn := row[0]
		code := strings.TrimPrefix(btn.CallbackData, "da:")
		provider, ok := providerOf(code)
		if !ok || provider == "" {
			t.Errorf("providerOf(%q) failed for button callback %s", code, btn.CallbackData)
		}
	}
}
