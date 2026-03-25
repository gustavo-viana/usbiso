package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"usbiso/validations"
)

type BlockDevice struct {
	Name  string `json:"name"`
	Size  string `json:"size"`
	Model string `json:"model"`
	RM    bool   `json:"rm"`
	Type  string `json:"type"`
	Tran  string `json:"tran"`
}

type LSBLK struct {
	Blockdevices []BlockDevice `json:"blockdevices"`
}

// Executa a gravação da ISO
func execBurn(
	path, device string,
	progressText binding.String,
	progressValue binding.Float,
) error {
	cmd := exec.Command(
		"pkexec",
		"dd",
		"if="+path,
		"of="+device,
		"bs=4M",
		"status=progress",
		"oflag=sync",
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	totalSize := float64(fileInfo.Size())

	if err := cmd.Start(); err != nil {
		return err
	}

	re := regexp.MustCompile(`(\d+) bytes.* ([0-9.]+) s, ([0-9.]+) MB/s`)

	go func() {
		buf := make([]byte, 1024)
		var line string
		var lastUpdate time.Time

		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])

				for _, c := range chunk {
					if c == '\r' {
						// linha completa do dd
						m := re.FindStringSubmatch(line)

						if len(m) >= 4 {
							bytesWritten, _ := strconv.ParseFloat(m[1], 64)
							speed, _ := strconv.ParseFloat(m[3], 64)

							percent := bytesWritten / totalSize

							var remaining float64
							if speed > 0 {
								remaining = (totalSize - bytesWritten) / (speed * 1024 * 1024)
							}

							// suavização
							if time.Since(lastUpdate) >= 100*time.Millisecond {
								lastUpdate = time.Now()

								progressValue.Set(percent)
								progressText.Set(fmt.Sprintf(
									"%.1f%% | %.2f MB/s | %.0fs restante",
									percent*100,
									speed,
									remaining,
								))
							}
						}

						line = ""
					} else {
						line += string(c)
					}
				}
			}

			if err != nil {
				break
			}
		}
	}()

	return cmd.Wait()
}

// Busca os dispositivos USB
func getUSBDevices() ([]BlockDevice, error) {
	cmd := exec.Command("lsblk", "-J", "-o", "NAME,SIZE,MODEL,RM,TYPE,TRAN")
	var output bytes.Buffer
	cmd.Stdout = &output

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var data LSBLK
	err = json.Unmarshal(output.Bytes(), &data)
	if err != nil {
		return nil, err
	}

	var devices []BlockDevice

	for _, d := range data.Blockdevices {
		if d.RM && d.Type == "disk" && d.Tran == "usb" {
			devices = append(devices, d)
		}
	}

	return devices, nil
}

// Espaçamento UI
func VSpace(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

func main() {
	app := app.NewWithID("usbiso.app")
	w := app.NewWindow("usbiso")

	devices, err := getUSBDevices()
	if err != nil {
		log.Println("Erro ao listar dispositivos:", err)
	}

	var devicesToOptions []string
	deviceMap := make(map[string]string)

	for _, d := range devices {
		label := fmt.Sprintf("%s (%s)", d.Name, d.Size)
		value := "/dev/" + d.Name

		deviceMap[label] = value
		devicesToOptions = append(devicesToOptions, label)
	}

	var deviceSelected string
	selectedDevice := widget.NewSelect(devicesToOptions, func(selected string) {
		log.Println("Dispositivo selecionado:", selected)
		deviceSelected = deviceMap[selected]
	})

	// ISO
	var path string

	lblPath := widget.NewLabel("Nenhum arquivo selecionado")

	btnSelectISO := widget.NewButtonWithIcon("Selecionar ISO", theme.FolderOpenIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil {
				log.Println("BUSCA DA ISO")
				dialog.ShowError(err, w)
				return
			}

			if file == nil {
				return
			}

			path = file.URI().Path()

			// validação
			err = validations.ValidationFiles(path)
			if err != nil {
				log.Println("Validacao da ISO")
				dialog.ShowError(err, w)
				return
			}

			lblPath.SetText(path)
			log.Println("ISO válida:", path)

			defer file.Close()
		}, w)

		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".iso"}))
		fileDialog.Show()
	})

	//RUN
	progressText := binding.NewString()
	progressBarValue := binding.NewFloat()

	progressLabel := widget.NewLabelWithData(progressText)
	progressBar := widget.NewProgressBarWithData(progressBarValue)

	bntRunBurn := widget.NewButtonWithIcon("RUN", theme.ConfirmIcon(), func() {
		go func() {
			err := execBurn(path, deviceSelected, progressText, progressBarValue)
			if err != nil {
				log.Println("Rotina da gravacao")
				dialog.ShowError(err, w)
				return
			}

			progressText.Set("Concluído")
			progressBarValue.Set(1)
		}()
	})

	// TEXTOS
	describeISO := canvas.NewText("SELECT ISO FILE", color.White)
	describeISO.Alignment = fyne.TextAlignCenter
	describeISO.TextStyle = fyne.TextStyle{Italic: true}

	describeUSB := canvas.NewText("SELECT USB DEVICE", color.White)
	describeUSB.Alignment = fyne.TextAlignCenter
	describeUSB.TextStyle = fyne.TextStyle{Italic: true}

	describeRUN := canvas.NewText("RUN", color.White)
	describeRUN.Alignment = fyne.TextAlignCenter
	describeRUN.TextStyle = fyne.TextStyle{Italic: true}

	//SEÇÕES
	isoSection := container.NewVBox(
		describeISO,
		VSpace(10),
		btnSelectISO,
		VSpace(10),
		lblPath,
	)

	usbSection := container.NewVBox(
		describeUSB,
		VSpace(10),
		selectedDevice,
	)

	runSection := container.NewVBox(
		describeRUN,
		VSpace(10),
		progressBar,
		progressLabel,
		bntRunBurn,
	)

	//LAYOUT PRINCIPAL
	content := container.NewVBox(
		isoSection,
		VSpace(30),
		usbSection,
		VSpace(30),
		runSection,
	)

	w.Resize(fyne.NewSize(500, 500))
	w.SetContent(content)
	w.ShowAndRun()
}
