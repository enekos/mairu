package mask

import "testing"

func TestValidIPv4(t *testing.T) {
	yes := []string{"10.0.0.5", "192.168.1.1", "8.8.8.8"}
	no := []string{"1.2.3.4.5", "999.0.0.1", "01.02.03.04", "0.0.0.0"}
	for _, s := range yes {
		if !validIPv4(s) {
			t.Errorf("%q should validate", s)
		}
	}
	for _, s := range no {
		if validIPv4(s) {
			t.Errorf("%q should be rejected", s)
		}
	}
}

func TestValidLuhn(t *testing.T) {
	yes := []string{"4111 1111 1111 1111", "5555-5555-5555-4444", "378282246310005"}
	no := []string{"1234 5678 9012 3456", "4111 1111 1111 1112", "abcd"}
	for _, s := range yes {
		if !validLuhn(s) {
			t.Errorf("%q should pass Luhn", s)
		}
	}
	for _, s := range no {
		if validLuhn(s) {
			t.Errorf("%q should fail Luhn", s)
		}
	}
}

func TestValidIBAN(t *testing.T) {
	yes := []string{"DE89370400440532013000", "GB82WEST12345698765432"}
	no := []string{"DE00370400440532013000", "XX00", "toolong" + "AAAA"}
	for _, s := range yes {
		if !validIBAN(s) {
			t.Errorf("%q should validate as IBAN", s)
		}
	}
	for _, s := range no {
		if validIBAN(s) {
			t.Errorf("%q should be rejected as IBAN", s)
		}
	}
}

func TestValidJWT(t *testing.T) {
	// eyJhbGciOiJIUzI1NiJ9 decodes to {"alg":"HS256"}
	if !validJWT("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature") {
		t.Error("valid jwt rejected")
	}
	if validJWT("abc.def.ghi") {
		t.Error("non-base64 header accepted")
	}
	if validJWT("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.") {
		t.Error("empty signature accepted")
	}
	if !validJWT("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.xyz") {
		t.Error("short but non-empty signature should validate (prevents payload leak)")
	}
}

func TestValidEth(t *testing.T) {
	if !validEth("0xAbCdef0123456789abcdef0123456789abcdef01") {
		t.Error("valid eth rejected")
	}
	if validEth("0x" + "G" + "0123456789abcdef0123456789abcdef0123456") {
		t.Error("non-hex accepted")
	}
}
