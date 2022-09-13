package phonehome

import (
	"bytes"

	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

/*
DTMFEncodeTable is the mapping between a symbol/number and corresponding DTMF character sequences.
This is the partial inverse of DTMFDecodeTable, suffix character 0 from each character sequence is
an indication of end of a sequence, sought by DTMFDecode.
*/
var DTMFEncodeTable = map[rune]string{
	' ': `0`,
	'!': `1110`, '@': `1120`, '#': `1130`, '$': `1140`, '%': `1150`, '^': `1160`, '&': `1170`, '*': `1180`, '(': `1190`,
	'`': `1210`, '~': `1220`, ')': `1230`, '-': `1240`, '_': `1250`, '=': `1260`, '+': `1270`, '[': `1280`, '{': `1290`,
	']': `1310`, '}': `1320`, '\\': `1330`, '|': `1340`, ';': `1350`, ':': `1360`, '\'': `1370`, '"': `1380`, ',': `1390`,
	'<': `1410`, '.': `1420`, '>': `1430`, '/': `1440`, '?': `1450`,

	toolbox.SubjectReportSerialisedFieldSeparator: `1460`,
	toolbox.SubjectReportSerialisedLineSeparator:  `1470`,

	'0': `10`, '1': `110`, '2': `120`, '3': `130`, '4': `140`, '5': `150`, '6': `160`, '7': `170`, '8': `180`, '9': `190`,
}

/*
EncodeToDTMF encodes the input string into a form acceptable by the DNS daemon's query processor
for running an application command. Specifically, the Latin letters remain in-place, numbers and
symbols are substituted for DTMF sequences. Later on, when the DNS daemon picks up the encoded
string from a query, it will decode the DTMF sequences to recover the original.

The return value must be further split apart into DNS labels of no more than 63 characters each.
*/
func EncodeToDTMF(in string) string {
	var out bytes.Buffer

	for _, c := range in {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			out.WriteRune(c)
		} else if dtmfSeq, exists := DTMFEncodeTable[c]; exists {
			out.WriteString(dtmfSeq)
		} else {
			lalog.DefaultLogger.Info("", nil, "do not know how to encode character '%c'", c)
		}
	}
	return out.String()
}

/*
GetDNSQuery constructs a DNS name ready to be queried, the name consists of the input app command
with DTMF encoded sequences, the domain name, and the mandatory command query prefix.
If necessary, the app command will be truncated to fit into the maximum length of a name query.
*/
func GetDNSQuery(appCmd, domainName string) string {
	encodedAppCmd := EncodeToDTMF(appCmd)
	var out bytes.Buffer

	// Be on the safe side and avoid filling up all 253 characters of a DNS name
	labelsCapacity := 246 - len(domainName)
	out.WriteRune(dnsd.ToolboxCommandPrefix)
	out.WriteRune('.')
	for {
		// Be on the safe side and avoid filling up all 63 characters of a DNS label
		labelLen := 60
		if l := len(encodedAppCmd); l < labelLen {
			labelLen = l
		}
		if labelsCapacity < labelLen {
			labelLen = labelsCapacity
		}
		if labelLen < 1 {
			break
		}
		out.WriteString(encodedAppCmd[:labelLen])
		out.WriteRune('.')
		encodedAppCmd = encodedAppCmd[labelLen:]
		labelsCapacity = labelsCapacity - labelLen - 1
	}
	out.WriteString(domainName)
	return out.String()
}
