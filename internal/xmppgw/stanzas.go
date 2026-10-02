package xmppgw

import (
	"encoding/xml"

	"mellium.im/xmpp/stanza"
)

const (
	nsMUC     = "http://jabber.org/protocol/muc"
	nsMUCUser = "http://jabber.org/protocol/muc#user"
	nsDelay   = "urn:xmpp:delay"
	nsVersion = "jabber:iq:version"
	nsPing    = "urn:xmpp:ping"
	nsRoster  = "jabber:iq:roster"
)

// Incoming stanzas. Only the parts the gateway cares about are decoded.

type inMessage struct {
	stanza.Message
	Body    string  `xml:"body"`
	Subject *string `xml:"subject"`
	Delay   *delay  `xml:"urn:xmpp:delay delay"`
}

type delay struct {
	Stamp string `xml:"stamp,attr"`
}

type inPresence struct {
	stanza.Presence
	MUCUser *mucUser     `xml:"http://jabber.org/protocol/muc#user x"`
	Error   *stanzaError `xml:"error"`
}

type mucUser struct {
	Item   *mucItem    `xml:"item"`
	Status []mucStatus `xml:"status"`
}

type mucItem struct {
	Nick string `xml:"nick,attr"`
	JID  string `xml:"jid,attr"`
}

type mucStatus struct {
	Code int `xml:"code,attr"`
}

func (u *mucUser) codes() map[int]bool {
	codes := map[int]bool{}
	if u == nil {
		return codes
	}
	for _, s := range u.Status {
		codes[s.Code] = true
	}
	return codes
}

type stanzaError struct {
	Type  string `xml:"type,attr"`
	Inner string `xml:",innerxml"`
}

type inIQ struct {
	stanza.IQ
	Version *struct{} `xml:"jabber:iq:version query"`
	Ping    *struct{} `xml:"urn:xmpp:ping ping"`
	Roster  *struct{} `xml:"jabber:iq:roster query"`
}

// Outgoing stanzas.

type outMessage struct {
	XMLName xml.Name `xml:"message"`
	ID      string   `xml:"id,attr,omitempty"`
	To      string   `xml:"to,attr"`
	Type    string   `xml:"type,attr"`
	Body    string   `xml:"body"`
}

type outPresence struct {
	XMLName xml.Name `xml:"presence"`
	ID      string   `xml:"id,attr,omitempty"`
	To      string   `xml:"to,attr,omitempty"`
	Type    string   `xml:"type,attr,omitempty"`
	MUC     *mucJoin `xml:"http://jabber.org/protocol/muc x,omitempty"`
}

type mucJoin struct {
	XMLName  xml.Name    `xml:"http://jabber.org/protocol/muc x"`
	Password string      `xml:"password,omitempty"`
	History  *mucHistory `xml:"history"`
}

type mucHistory struct {
	MaxStanzas int `xml:"maxstanzas,attr"`
}

type versionResult struct {
	XMLName xml.Name `xml:"iq"`
	ID      string   `xml:"id,attr"`
	To      string   `xml:"to,attr,omitempty"`
	Type    string   `xml:"type,attr"`
	Query   struct {
		XMLName xml.Name `xml:"jabber:iq:version query"`
		Name    string   `xml:"name,omitempty"`
		Version string   `xml:"version,omitempty"`
		OS      string   `xml:"os,omitempty"`
	}
}

type emptyResult struct {
	XMLName xml.Name `xml:"iq"`
	ID      string   `xml:"id,attr"`
	To      string   `xml:"to,attr,omitempty"`
	Type    string   `xml:"type,attr"`
}
