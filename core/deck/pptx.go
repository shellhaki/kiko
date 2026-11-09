package deck

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"strconv"
	"strings"
)

func escape(value string) string {
	var out bytes.Buffer
	xml.EscapeText(&out, []byte(value))
	return out.String()
}

const namespaces = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
const group = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`
const relationships = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`

func relationship(id, kind, target string) string {
	return `<Relationship Id="` + id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + kind + `" Target="` + escape(target) + `"/>`
}
func textShape(id int, x, y, w, h, size int, lines []string, bold, bullets bool) string {
	var text strings.Builder
	for _, line := range lines {
		props := ""
		if bullets {
			props = `<a:pPr marL="228600" indent="-171450"><a:buChar char="•"/></a:pPr>`
		}
		weight := ""
		if bold {
			weight = ` b="1"`
		}
		text.WriteString(`<a:p>` + props + `<a:r><a:rPr lang="en-US" sz="` + strconv.Itoa(size*100) + `"` + weight + `><a:solidFill><a:srgbClr val="EEEEEE"/></a:solidFill><a:latin typeface="Arial"/></a:rPr><a:t>` + escape(line) + `</a:t></a:r><a:endParaRPr lang="en-US" sz="` + strconv.Itoa(size*100) + `"/></a:p>`)
	}
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Text %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0"><a:normAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody></p:sp>`, id, id, x, y, w, h, text.String())
}
func rectangle(id, x, y, w, h int, fill string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Chart bar"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></p:spPr></p:sp>`, id, x, y, w, h, fill)
}
func PPTX(deck Deck, resolveImage func(string) ([]byte, error)) ([]byte, error) {
	if err := Prepare(&deck); err != nil {
		return nil, err
	}
	parts := map[string][]byte{}
	types := `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/>`
	override := func(name, kind string) {
		types += `<Override PartName="/` + name + `" ContentType="application/vnd.openxmlformats-officedocument.presentationml.` + kind + `+xml"/>`
	}
	override("ppt/presentation.xml", "presentation.main")
	override("ppt/slideMasters/slideMaster1.xml", "slideMaster")
	override("ppt/slideLayouts/slideLayout1.xml", "slideLayout")
	override("ppt/notesMasters/notesMaster1.xml", "notesMaster")
	types += `<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`
	parts["_rels/.rels"] = []byte(relationships + relationship("presentation", "officeDocument", "ppt/presentation.xml") + "</Relationships>")
	master := `<p:sldMaster ` + namespaces + `><p:cSld><p:spTree>` + group + `</p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="layout"/></p:sldLayoutIdLst><p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles></p:sldMaster>`
	parts["ppt/slideMasters/slideMaster1.xml"] = []byte(master)
	parts["ppt/slideMasters/_rels/slideMaster1.xml.rels"] = []byte(relationships + relationship("layout", "slideLayout", "../slideLayouts/slideLayout1.xml") + relationship("theme", "theme", "../theme/theme1.xml") + "</Relationships>")
	parts["ppt/slideLayouts/slideLayout1.xml"] = []byte(`<p:sldLayout ` + namespaces + ` type="blank" preserve="1"><p:cSld name="Blank"><p:spTree>` + group + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`)
	parts["ppt/slideLayouts/_rels/slideLayout1.xml.rels"] = []byte(relationships + relationship("master", "slideMaster", "../slideMasters/slideMaster1.xml") + "</Relationships>")
	parts["ppt/notesMasters/notesMaster1.xml"] = []byte(`<p:notesMaster ` + namespaces + `><p:cSld><p:spTree>` + group + `</p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/><p:notesStyle/></p:notesMaster>`)
	parts["ppt/notesMasters/_rels/notesMaster1.xml.rels"] = []byte(relationships + relationship("theme", "theme", "../theme/theme1.xml") + "</Relationships>")
	theme := `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Kiko"><a:themeElements><a:clrScheme name="Kiko">`
	for _, item := range [][2]string{{"dk1", "181818"}, {"lt1", "EEEEEE"}, {"dk2", "262626"}, {"lt2", "B5B5B5"}, {"accent1", "7950CC"}, {"accent2", "8ACDA6"}, {"accent3", "E4BF7B"}, {"accent4", "F29999"}, {"accent5", "B48AF7"}, {"accent6", "A1A1A1"}, {"hlink", "B48AF7"}, {"folHlink", "7950CC"}} {
		theme += `<a:` + item[0] + `><a:srgbClr val="` + item[1] + `"/></a:` + item[0] + `>`
	}
	theme += `</a:clrScheme><a:fontScheme name="Arial"><a:majorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="Kiko"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="25400"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="38100"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`
	parts["ppt/theme/theme1.xml"] = []byte(theme)
	presentation := `<p:presentation ` + namespaces + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="master"/></p:sldMasterIdLst><p:notesMasterIdLst><p:notesMasterId r:id="notes"/></p:notesMasterIdLst><p:sldIdLst>`
	rels := relationships + relationship("master", "slideMaster", "slideMasters/slideMaster1.xml") + relationship("notes", "notesMaster", "notesMasters/notesMaster1.xml")
	for index, slide := range deck.Slides {
		number := strconv.Itoa(index + 1)
		override("ppt/slides/slide"+number+".xml", "slide")
		override("ppt/notesSlides/notesSlide"+number+".xml", "notesSlide")
		presentation += fmt.Sprintf(`<p:sldId id="%d" r:id="slide%s"/>`, 256+index, number)
		rels += relationship("slide"+number, "slide", "slides/slide"+number+".xml")
		shapes := textShape(2, 600000, 500000, 10900000, 1100000, 32, []string{slide.Title}, true, false)
		textSize := 22
		total := 0
		for _, point := range append(append([]string{}, slide.Body...), append(slide.Left, slide.Right...)...) {
			total += len([]rune(point))
		}
		if total > 550 {
			textSize = 16
		}
		switch slide.Layout {
		case "title":
			shapes = textShape(2, 800000, 1800000, 10500000, 1700000, 44, []string{slide.Title}, true, false) + textShape(3, 800000, 3900000, 10500000, 1700000, 24, slide.Body, false, false)
		case "bullets":
			shapes += textShape(3, 750000, 2000000, 10600000, 4100000, textSize, slide.Body, false, true)
		case "columns":
			shapes += textShape(3, 650000, 2000000, 5100000, 4100000, textSize, slide.Left, false, true) + textShape(4, 6400000, 2000000, 5100000, 4100000, textSize, slide.Right, false, true)
		case "image":
			if resolveImage == nil {
				return nil, fmt.Errorf("image resolver is unavailable")
			}
			data, err := resolveImage(slide.ImageID)
			if err != nil {
				return nil, err
			}
			config, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			parts["ppt/media/image"+number+".png"] = data
			w, h := float64(8500000), float64(3800000)
			ratio := math.Min(w/float64(config.Width), h/float64(config.Height))
			w = float64(config.Width) * ratio
			h = float64(config.Height) * ratio
			shapes += fmt.Sprintf(`<p:pic><p:nvPicPr><p:cNvPr id="3" name="Image"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="image"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="%d" y="1800000"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`, int((12192000-w)/2), int(w), int(h))
			shapes += textShape(4, 650000, 5900000, 10900000, 650000, 16, slide.Body, false, false)
		case "chart":
			low, high := 0.0, 0.0
			for _, v := range slide.Chart.Values {
				low = math.Min(low, v)
				high = math.Max(high, v)
			}
			if high == low {
				high = low + 1
			}
			step := 10000000 / len(slide.Chart.Values)
			zero := 1900000 + int(high/(high-low)*3400000)
			for i, v := range slide.Chart.Values {
				y := 1900000 + int((high-v)/(high-low)*3400000)
				top := min(y, zero)
				height := max(1000, int(math.Abs(float64(y-zero))))
				x := 850000 + i*step
				shapes += rectangle(10+i*3, x, top, step*2/3, height, "7950CC")
				shapes += textShape(11+i*3, x, 5400000, step*9/10, 900000, 11, []string{slide.Chart.Labels[i]}, false, false)
				shapes += textShape(12+i*3, x, top-300000, step*9/10, 300000, 12, []string{strconv.FormatFloat(v, 'f', -1, 64)}, false, false)
			}
		}
		shapes += textShape(1000, 11300000, 6400000, 300000, 300000, 10, []string{number}, false, false)
		parts["ppt/slides/slide"+number+".xml"] = []byte(`<p:sld ` + namespaces + `><p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="181818"/></a:solidFill><a:effectLst/></p:bgPr></p:bg><p:spTree>` + group + shapes + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
		slideRels := relationships + relationship("layout", "slideLayout", "../slideLayouts/slideLayout1.xml") + relationship("notes", "notesSlide", "../notesSlides/notesSlide"+number+".xml")
		if slide.Layout == "image" {
			slideRels += relationship("image", "image", "../media/image"+number+".png")
		}
		parts["ppt/slides/_rels/slide"+number+".xml.rels"] = []byte(slideRels + "</Relationships>")
		notes := `<p:notes ` + namespaces + `><p:cSld><p:spTree>` + group + textShape(2, 0, 0, 6000000, 8000000, 12, []string{slide.Notes}, false, false) + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:notes>`
		parts["ppt/notesSlides/notesSlide"+number+".xml"] = []byte(notes)
		parts["ppt/notesSlides/_rels/notesSlide"+number+".xml.rels"] = []byte(relationships + relationship("slide", "slide", "../slides/slide"+number+".xml") + relationship("master", "notesMaster", "../notesMasters/notesMaster1.xml") + "</Relationships>")
	}
	parts["ppt/presentation.xml"] = []byte(presentation + `</p:sldIdLst><p:sldSz cx="12192000" cy="6858000" type="screen16x9"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`)
	parts["ppt/_rels/presentation.xml.rels"] = []byte(rels + "</Relationships>")
	parts["[Content_Types].xml"] = []byte(types + "</Types>")
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, data := range parts {
		file, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = file.Write(data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
