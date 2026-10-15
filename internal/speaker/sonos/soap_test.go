package sonos

import "testing"

func TestSoapFault_UPnPError(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><faultstring>Transport unavailable</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>701</errorCode></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
	got := soapFault(body)
	if got != "UPnP error code 701: Transport unavailable" {
		t.Fatalf("soapFault = %q", got)
	}
}

func TestSoapFault_RegexFallback(t *testing.T) {
	body := []byte(`<s:Fault><s:faultstring>Action not supported</s:faultstring></s:Fault>`)
	got := soapFault(body)
	if got != "Action not supported" {
		t.Fatalf("soapFault = %q", got)
	}
}

func TestSoapFault_BodySnippet(t *testing.T) {
	body := []byte("not xml at all")
	got := soapFault(body)
	if got != "Response: not xml at all" {
		t.Fatalf("soapFault = %q", got)
	}
}
