package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/schaepher/mobile/app"
	"github.com/schaepher/mobile/event/lifecycle"
	"github.com/schaepher/mobile/event/paint"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"
)

// HID report for "A" key press and release
var (
	hidKeyPressA  = []byte{0xA1, 0x01, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00} // Press 'A'
	hidKeyRelease = []byte{0xA1, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00} // Release
)

// Global notifier for BLE notifications
var notifier ble.Notifier

func main() {
	app.Main(func(a app.App) {
		// No OpenGL context handling if not needed
		for e := range a.Events() {
			switch e := a.Filter(e).(type) {
			case lifecycle.Event:
				switch e.Crosses(lifecycle.StageVisible) {
				case lifecycle.CrossOn:
					// Start BLE service and HTTP server
					go startBLEService()
					go startHTTPServer()
				case lifecycle.CrossOff:
					// Stop the app
					log.Println("App stopped")
					return
				}
			case paint.Event:
				// Handle drawing logic here, if needed
				a.Publish()
			}
		}
	})
}

func startBLEService() {
	// Initialize BLE device
	d, err := dev.NewDevice("default")
	if err != nil {
		log.Fatalf("Failed to initialize BLE device: %v", err)
	}
	ble.SetDefaultDevice(d)

	// Create HID service
	hidSvc := ble.NewService(ble.MustParse("1812"))            // HID Service UUID
	reportChar := ble.NewCharacteristic(ble.MustParse("2A4D")) // Report Characteristic UUID
	reportChar.HandleNotify(&notifyHandler{})
	hidSvc.AddCharacteristic(reportChar)
	ble.AddService(hidSvc)

	// Start advertising
	log.Println("Starting BLE advertisement...")
	err = ble.AdvertiseNameAndServices(context.Background(), "BLE Keyboard", hidSvc.UUID)
	if err != nil {
		log.Fatalf("Failed to start advertising: %v", err)
	}
}

func startHTTPServer() {
	http.HandleFunc("/enter", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Received HTTP request: Sending HID report")
		if notifier == nil {
			http.Error(w, "Notifier not initialized", http.StatusInternalServerError)
			return
		}
		if err := sendHIDReport(hidKeyPressA); err != nil {
			http.Error(w, "Failed to send key press", http.StatusInternalServerError)
			return
		}
		if err := sendHIDReport(hidKeyRelease); err != nil {
			http.Error(w, "Failed to release key", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Key A sent"))
	})
	log.Println("HTTP server running at http://localhost:8080/enter")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// notifyHandler implements ble.NotifyHandler
type notifyHandler struct{}

func (h *notifyHandler) ServeNotify(req ble.Request, n ble.Notifier) {
	log.Println("Notifier initialized")
	notifier = n // Save the notifier for future use
	for {
		select {
		case <-n.Context().Done():
			log.Println("Notifier closed")
			notifier = nil
			return
		}
	}
}

// sendHIDReport sends the specified HID report using the notifier
func sendHIDReport(report []byte) error {
	if notifier == nil {
		return errors.New("notifier not initialized")
	}
	_, err := notifier.Write(report)
	return err
}
