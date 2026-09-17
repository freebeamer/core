package types

import "encoding/xml"

type XDFXMLDocument struct {
	XMLName   xml.Name         `xml:"XDFFORMAT"`
	Version   string           `xml:"version,attr"`
	Header    XDFXMLHeader     `xml:"XDFHEADER"`
	Tables    []XDFXMLTable    `xml:"XDFTABLE"`
	Flags     []XDFXMLFlag     `xml:"XDFFLAG"`
	Constants []XDFXMLConstant `xml:"XDFCONSTANT"`
}

type XDFXMLHeader struct {
	Title       string          `xml:"title"`
	Description string          `xml:"description"`
	Author      string          `xml:"author"`
	BaseOffset  XDFXMLElement   `xml:"BASEOFFSET"`
	Defaults    XDFXMLElement   `xml:"DEFAULTS"`
	Regions     []XDFXMLElement `xml:"REGION"`
	Categories  []XDFXMLElement `xml:"CATEGORY"`
}

type XDFXMLTable struct {
	Attrs       []xml.Attr      `xml:",any,attr"`
	Title       string          `xml:"title"`
	Description string          `xml:"description"`
	Categories  []XDFXMLElement `xml:"CATEGORYMEM"`
	Axes        []XDFXMLAxis    `xml:"XDFAXIS"`
}

type XDFXMLFlag struct {
	Attrs       []xml.Attr      `xml:",any,attr"`
	Title       string          `xml:"title"`
	Description string          `xml:"description"`
	Categories  []XDFXMLElement `xml:"CATEGORYMEM"`
	Embedded    XDFXMLElement   `xml:"EMBEDDEDDATA"`
	Math        XDFXMLElement   `xml:"MATH"`
	Mask        string          `xml:"mask"`
}

// XDFXMLConstant is a scalar value declared outside any table: a single
// addressed value with its own units/decimalpl/outputtype, structurally a
// hybrid of XDFXMLTable/XDFXMLFlag (title/description/CATEGORYMEM) and
// XDFXMLAxis (units/decimalpl/outputtype), per two independent open-source
// XDF implementations (OpenEEC's XdfScalar, a2l2xdf's XDFCONSTANT builder).
type XDFXMLConstant struct {
	Attrs         []xml.Attr      `xml:",any,attr"`
	Title         string          `xml:"title"`
	Description   string          `xml:"description"`
	Categories    []XDFXMLElement `xml:"CATEGORYMEM"`
	Embedded      XDFXMLElement   `xml:"EMBEDDEDDATA"`
	Units         string          `xml:"units"`
	DecimalPlaces string          `xml:"decimalpl"`
	OutputType    string          `xml:"outputtype"`
	Min           string          `xml:"min"`
	Max           string          `xml:"max"`
	Math          XDFXMLElement   `xml:"MATH"`
}

type XDFXMLAxis struct {
	Attrs         []xml.Attr      `xml:",any,attr"`
	Units         string          `xml:"units"`
	IndexCount    string          `xml:"indexcount"`
	DecimalPlaces string          `xml:"decimalpl"`
	OutputType    string          `xml:"outputtype"`
	Min           string          `xml:"min"`
	Max           string          `xml:"max"`
	Embedded      XDFXMLElement   `xml:"EMBEDDEDDATA"`
	Math          XDFXMLElement   `xml:"MATH"`
	Labels        []XDFXMLElement `xml:"LABEL"`
}

type XDFXMLElement struct {
	Attrs []xml.Attr `xml:",any,attr"`
}
