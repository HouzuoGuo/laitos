package toolbox

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// SARContact has contact information about a search-and-rescue institution.
type SARContact struct {
	RegionAbbr string // RegionAbbr is the abbreviation of region name managed/represented by the institution.

	/*
		Type is the institution type:
		GEOS - the home of the International Emergency Response Coordination Centre (IERCC)
		ARCC - aeronautical rescue coordination centre
		MRCC - marine rescue coordination centre
		NMOC - national maritime operation centre
		MSAR - maritime search and rescue
		ASAR - aeronautical search and rescue
		JRCC - joint rescue coordination centre
		MCC - mission control centre
		RCC - rescue coordination centre
	*/
	Type string

	Telephone string // Telephone number of the institution. This is only for voice calls, not for SMS texts.
	Email     string // Email address of the institution.
}

// SARContacts are pre-programmed SAR contact information.
var SARContacts = []SARContact{
	// http://geosresponse.com/contact-us.html
	{"INT", "GEOS", "+19365823190", ""},
	{"US", "GEOS", "+18554442937", ""},
	// http://www.cad.gov.hk/english/notifyairacc.html
	{"HK", "ARCC", "+85229106821", "aid@cad.gov.hk"},
	// http://www.mardep.gov.hk/en/pub_services/ocean/home.html
	{"HK", "MRCC", "+85222337999", "hkmrcc@mardep.gov.hk"},
	// https://www.admiralty.co.uk/WeeklyNMs/Year%20-%202017/28wknm17_week28_2017.pdf
	{"UK", "ARCC", "+441343836001", "ukarcc@hmcg.gov.uk"},
	{"UK", "MCC", "+441343820902", "ukmcc@hmcg.gov.uk"},
	{"UK", "NMOC", "+443443820025", ""},
	// https://www.amsa.gov.au/emergency-contacts/index.asp
	{"AU", "MSAR", "+61262306811", "rccaus@amsa.gov.au"},
	{"AU", "ASAR", "+61262306899", "rccaus@amsa.gov.au"},
	// https://cospas-sarsat.int/en/contacts-pro/contacts-details-all?view=contact_details
	{"CA", "JRCC", "+19024278200", "jrcchalifax@sarnet.dnd.ca"},
	{"CN", "MCC", "+861065293298", "cnmcc@mail.eastnet.com.cn"},
	{"JP", "MCC", "+81335919000", "op@kaiho.mlit.go.jp"},
	{"GR", "JRCC", "+302104112500", "jrccpgr@yen.gr"},
	// http://www.raja.fi/contact/contact
	{"FI", "MRCC", "+3582941000", "mrcc@raja.fi"},
	// http://www.smrcc.ru/
	{"RU", "MRCC", "+74956261052", "odsmrcc@morflot.ru"},
	// https://www.inmarsat.com/services/safety/maritime-rescue-co-ordination-centres/
	{"CN", "MRCC", "+861065292221", "cnmrcc@mot.gov.cn"},
	{"IL", "RCC", "+97248632145", "rcc@mot.gov.il"},
	{"NO", "JRCC", "+4751517000", "operations@jrcc-stavanger.no"},
	{"KR", "MRCC", "+82328352594", "mrcckorea@korea.kr"},
	{"US", "ACC", "+17573986700", "lantwatch@uscg.mil"},
}

// GetAllSAREmails returns all SAR institution Email addresses.
func GetAllSAREmails() []string {
	ret := make([]string, 0, len(SARContacts))
	for _, contact := range SARContacts {
		if contact.Email != "" {
			ret = append(ret, contact.Email)
		}
	}
	return ret
}

/*
PublicContact serves contact information from well known institutions. At this moment, it only serves pre-programmed
information about SAR (search-and-rescue) institutions.

Note to developer: in the future, consider serving more public institution contacts.
*/
type PublicContact struct {
	textRecords []string // textRecords are contact entries organised into human-readable text, one line per entry.
}

func (cs *PublicContact) IsConfigured() bool {
	return len(SARContacts) > 0
}

func (cs *PublicContact) SelfTest() error {
	if cs.textRecords == nil || len(cs.textRecords) == 0 {
		return errors.New("PublicContact.SelfTest: contact info is unavailable")
	}
	return nil
}

func (cs *PublicContact) Initialise() error {
	// Format all SAR contacts into text, one contact per line.
	cs.textRecords = make([]string, len(SARContacts))
	for i, entry := range SARContacts {
		// Convert to lower case for easier search
		cs.textRecords[i] = strings.ToLower(fmt.Sprintf("%s %s %s %s", entry.RegionAbbr, entry.Type, entry.Telephone, entry.Email))
	}
	return nil
}

func (cs *PublicContact) Trigger() Trigger {
	return ".c"
}

func (cs *PublicContact) Execute(cmd Command) *Result {
	// Return all entries if there is no search term, therefore do not return the error.
	if err := cmd.Trim(); err != nil {
		return &Result{Error: nil, Output: strings.Join(cs.textRecords, "\n")}
	}
	// Do not allow search term to be excessively long to avoid a potential
	if len(cmd.Content) > 128 {
		cmd.Content = cmd.Content[:128]
	}
	// Look for the search term among contact information lines
	for i := len(cmd.Content); i > 1; i-- {
		searchTerm := strings.TrimSpace(strings.ToLower(cmd.Content[:i]))
		var resultOut bytes.Buffer
		for _, line := range cs.textRecords {
			if strings.Contains(line, searchTerm) {
				resultOut.WriteString(line)
				resultOut.WriteRune('\n')
			}
		}
		// If the search term yields something, return the content.
		if resultOut.Len() > 0 {
			return &Result{Error: nil, Output: resultOut.String()}
		}
		// If the search does not yield anything, continue searching by removing a suffix character from search term.
	}
	// If search yields nothing, return all entries.
	return &Result{Error: nil, Output: strings.Join(cs.textRecords, "\n")}
}
