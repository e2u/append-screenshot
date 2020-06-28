package main

import (
	"flag"
	"github.com/e2u/e2util/e2env"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"strings"
)

var (
	inputFiles string
	outputFile string
	colNum     int
)

const (
	// 某行(y)的黑色和非黑色的比例阈值，用 notBlack / black 得到,如果小於 colorThreshold 設定，則這行還算是純黑色
	colorThreshold = 0.2
	// 連續多少行黑色比例不才不算邊框,比如某行的黑色和非黑色的比例大與 colorThreshold
	borderThreshold = 20
)

func main() {

	e2env.EnvStringVar(&inputFiles, "input", "", "input images,separate by a comma")
	e2env.EnvStringVar(&outputFile, "output", "", "output image")
	e2env.EnvIntVar(&colNum, "col", 2, "append image column number")

	flag.Parse()

	if len(inputFiles) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	if colNum <= 0 {
		colNum = 1
	}

	outFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer outFile.Close()

	imageFiles := strings.Split(inputFiles, ",")
	var srcImages []*image.NRGBA64
	for idx := range imageFiles {
		sf, err := os.Open(strings.TrimSpace(imageFiles[idx]))
		if err != nil {
			log.Fatal(err.Error())
		}
		si, _, err := image.Decode(sf)
		if err != nil {
			log.Fatal(err.Error())
		}
		srcImages = append(srcImages, corpImage(si.(*image.NRGBA64)))
		sf.Close()
	}

	outImage := appendImages(colNum, srcImages...)
	png.Encode(outFile, outImage)
}

// append 圖片拼接,按順序拼接多張圖片
// 先計算底圖的尺寸
// 需要找到最寬的圖片 x colNum 作為低圖的寬
// 圖片數量 / colNum 作為圖片的高
// 生成底圖後，按順序將圖片貼到底圖輸出
// colNum: 沒行有幾張圖片
func appendImages(colNum int, images ...*image.NRGBA64) *image.NRGBA64 {
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
		// log.Println("current idx,x,y", idx, currX, currY)
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

// corp 從源圖片截取部分並輸出,自動截取
func corpImage(srcImage *image.NRGBA64) *image.NRGBA64 {
	srcWidth, rscHeight := srcImage.Bounds().Max.X, srcImage.Bounds().Max.Y
	//log.Printf("origin image srcWidth x rscHeight: %v x %v\n", srcWidth, rscHeight)

	//var notBorderBeginY, notBorderEndY int
	// 存儲所有行的黑色和非黑色的比例
	var rows []float64
	for h := 0; h <= rscHeight; h++ {
		var black, notBlack int
		for w := 0; w <= rscHeight; w++ {
			r, g, b, _ := srcImage.At(w, h).RGBA()
			if r+g+b == 0 {
				black++
			} else {
				notBlack++
			}
		}
		rows = append(rows, float64(notBlack)/float64(black))
	}
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
	cropImageWidth, cropImageHeight := srcWidth, borderBackwardBegin-borderForwardEnd
	//log.Printf("crop image srcWidth x rscHeight: %v x %v\n", cropImageWidth, cropImageHeight)

	// 輸出最終圖片
	outImage := image.NewNRGBA64(image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: cropImageWidth, Y: cropImageHeight}})
	draw.Draw(outImage, outImage.Bounds(), srcImage, image.Point{X: 0, Y: borderForwardEnd}, draw.Src)
	return outImage
}
