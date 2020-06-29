package main

import (
	"flag"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/e2u/e2util/e2env"
	"github.com/sirupsen/logrus"
)

var (
	inputFiles string
	inputDir   string
	outputFile string
	colNum     int
)

const (
	// 某行(y)的純色和非純色的比例阈值，用 impure / pure 得到,如果小於 colorThreshold 設定，則這行還算是純色
	colorThreshold = 1.0
	// 連續多少行黑色比例不才不算邊框,比如某行的純色和非純色的比例大與 colorThreshold
	borderThreshold = 15
)

func main() {
	e2env.EnvStringVar(&inputDir, "input-dir", "", "input image dir path")
	e2env.EnvStringVar(&inputFiles, "input", "", "a comma separated list of image files")
	e2env.EnvStringVar(&outputFile, "output", "", "output image")
	e2env.EnvIntVar(&colNum, "col", 2, "append image column number")

	flag.Parse()

	if len(inputFiles) == 0 && len(inputDir) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	imageFiles := strings.Split(inputFiles, ",")

	_ = filepath.Walk(dirPath(inputDir), func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		exName := strings.ToLower(filepath.Ext(path))
		switch exName {
		case ".gif", ".png", ".jpg", ".jpeg", ".bmp":
			imageFiles = append(imageFiles, path)
		}
		return err
	})

	if len(imageFiles) == 0 {
		log.Fatal("input files can't not be empty")
	}

	if colNum <= 0 || len(imageFiles) == 1 {
		colNum = 1
	}

	outFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer outFile.Close()

	var srcImages []image.Image
	for idx := range imageFiles {
		if strings.TrimSpace(imageFiles[idx]) == "" {
			continue
		}
		si, err := readImageFile(imageFiles[idx])
		if err != nil {
			logrus.Fatal(err.Error())
		}
		//srcImages = append(srcImages, corpImageFR(si))
		srcImages = append(srcImages, corpImageF(si))
	}

	outImage := appendImages(colNum, srcImages...)
	_ = png.Encode(outFile, outImage)
}

// append 圖片拼接,按順序拼接多張圖片
// 先計算底圖的尺寸
// 需要找到最寬的圖片 x colNum 作為低圖的寬
// 圖片數量 / colNum 作為圖片的高
// 生成底圖後，按順序將圖片貼到底圖輸出
// colNum: 沒行有幾張圖片
func appendImages(colNum int, images ...image.Image) image.Image {
	var maxWidth, maxHeight int
	// 找到圖片裡的最寬和最高值
	for idx := range images {
		img := images[idx]
		width, height := img.Bounds().Max.X, img.Bounds().Max.Y
		if width > maxWidth {
			maxWidth = width
		}
		if height > maxHeight {
			maxHeight = height
		}
	}
	// 計算出行數,向上取整
	rowNum := int(math.Ceil(float64(len(images)) / float64(colNum)))

	// 計算出底圖尺寸並準備好底圖
	destWidth, destHeight := maxWidth*colNum, maxHeight*rowNum
	outImage := image.NewNRGBA64(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: destWidth, Y: destHeight}})
	// 按順序將圖片貼到底圖上，每次位移用最大尺寸

	var currX, currY int // 當前底圖的左上角
	for idx := range images {
		img := images[idx]
		draw.Draw(
			outImage,
			image.Rectangle{Min: image.Point{X: currX, Y: currY}, Max: image.Point{X: currX + img.Bounds().Max.X, Y: currY + img.Bounds().Max.Y}},
			img,
			image.Point{X: 0, Y: 0},
			draw.Src,
		)
		currX += maxWidth
		if (idx+1)%colNum == 0 { // move to next row
			currX = 0
			currY += maxHeight
		}
	}
	return outImage
}

// calcRowColorRate 按行計算圖片的純色和非純色的比例
func calcRowColorRate(srcImage image.Image) []float64 {
	srcWidth, rscHeight := srcImage.Bounds().Max.X, srcImage.Bounds().Max.Y
	// 存儲所有行的純色和非純的比例
	var rows []float64
	for h := 0; h <= rscHeight; h++ {
		var pure, impure int
		for w := 0; w <= srcWidth; w++ {
			r, g, b, _ := srcImage.At(w, h).RGBA()
			if r == g && r == b {
				pure++
			} else {
				impure++
			}

		}
		// logrus.Infof("pure=%v,impure=%v,rate=%v", pure, impure, float64(impure)/float64(pure))
		rows = append(rows, float64(impure)/float64(pure))
	}

	return rows
}

// corpImageF 單向查找邊框
func corpImageF(srcImage image.Image) image.Image {
	rows := calcRowColorRate(srcImage)
	// 分別獲取前導邊框的結束行和後導邊框的開始行，中間的就是需要截取的圖片
	var borderForwardBegin, borderForwardEnd, counter int
	// 連續非純色就能找到開始行
	for idx := range rows {
		if rows[idx] > colorThreshold {
			counter++
		} else {
			counter = 0
		}
		// 找到開始行
		if counter >= borderThreshold {
			borderForwardBegin = idx - borderThreshold
			break
		}
	}
	// 連續純色就找到結束行
	if borderForwardBegin < 0 {
		borderForwardBegin = 0
	}
	counter = 0
	for idx := borderForwardBegin; idx < len(rows); idx++ {
		if rows[idx] < colorThreshold {
			counter++
		} else {
			counter = 0
		}
		// 找到結束行
		if counter >= borderThreshold {
			borderForwardEnd = idx - borderThreshold
			break
		}
	}

	// logrus.Infof("border forward begin=%v,end=%v", borderForwardBegin, borderForwardEnd)
	// 計算出截取後的圖片尺寸
	srcWidth := srcImage.Bounds().Max.X
	cropImageWidth, cropImageHeight := srcWidth, borderForwardEnd-borderForwardBegin
	// logrus.Infof("crop width=%v,crop height=%v",cropImageWidth,cropImageHeight)
	// 輸出最終圖片
	outImage := image.NewNRGBA64(
		image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: cropImageWidth, Y: cropImageHeight},
		},
	)
	draw.Draw(outImage, outImage.Bounds(), srcImage, image.Point{X: 0, Y: borderForwardBegin}, draw.Src)
	return outImage
}

// corpImageFR 從源圖片截取部分並輸出,自動截取
// 雙向查找
func corpImageFR(srcImage image.Image) image.Image {
	rows := calcRowColorRate(srcImage)
	// 分別獲取前導邊框的結束行和後導邊框的開始行，中間的就是需要截取的圖片
	var borderForwardEnd, counter int
	for idx := range rows {
		if rows[idx] > colorThreshold {
			counter++
		} else {
			counter = 0
		}
		if counter >= borderThreshold {
			borderForwardEnd = idx - borderThreshold
			break
		}
	}
	var borderBackwardBegin int
	for idx := len(rows) - 1; idx > 0; idx-- {
		if rows[idx] > colorThreshold {
			counter++
		} else {
			counter = 0
		}
		if counter >= borderThreshold {
			borderBackwardBegin = idx + borderThreshold
			break
		}
	}
	// 計算出截取後的圖片尺寸
	srcWidth := srcImage.Bounds().Max.X
	cropImageWidth, cropImageHeight := srcWidth, borderBackwardBegin-borderForwardEnd
	// 輸出最終圖片
	outImage := image.NewNRGBA64(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: cropImageWidth, Y: cropImageHeight}})
	draw.Draw(outImage, outImage.Bounds(), srcImage, image.Point{X: 0, Y: borderForwardEnd}, draw.Src)
	return outImage
}

// readImageFile 讀取圖片文件
func readImageFile(file string) (*image.NRGBA64, error) {
	f, err := os.Open(strings.TrimSpace(file))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	i, format, err := image.Decode(f)
	if err != nil {
		logrus.Errorf("read image file error=%v", err.Error())
		return nil, err
	}
	logrus.Infof("read image file %v,format=%v", file, format)
	return convertImageToNRGBA64(i), err
}

// convertImageToNRGBA64 將圖片轉換成 *image.NRGBA64
func convertImageToNRGBA64(img image.Image) *image.NRGBA64 {
	m := image.NewNRGBA64(img.Bounds())
	draw.Draw(m, m.Bounds(), img, img.Bounds().Min, draw.Src)
	return m
}

func dirPath(inputDir string) string {
	if strings.HasPrefix(strings.TrimSpace(inputDir), "~/") {
		h, err := os.UserHomeDir()
		if err == nil {
			ap, err := filepath.Abs(filepath.Join(h, strings.Replace(inputDir, "~", "", 1)))
			if err != nil {
				return inputDir
			}
			return ap
		}
		return inputDir
	}
	return inputDir
}
