package official

import "testing"

// O rótulo precisa ser estável e não colidir com as contas de OAuth, cujo nome
// é o email. O email do vault vem desensitizado (m***1@gmail.com), então o id
// da conta é a única chave confiável.
func TestVaultLabel(t *testing.T) {
	cases := []struct {
		name string
		acc  VaultAccount
		want string
	}{
		{"account id", VaultAccount{AccountID: "1778373146", AccioID: "4500098242709"}, "1778373146@accio.desktop"},
		{"sem account id cai no accio id", VaultAccount{AccioID: "4500098242709"}, "4500098242709@accio.desktop"},
	}
	for _, c := range cases {
		if got := c.acc.VaultLabel(); got != c.want {
			t.Errorf("%s: VaultLabel() = %q, quer %q", c.name, got, c.want)
		}
	}
	if got := (VaultAccount{}).VaultLabel(); got == "" {
		t.Error("VaultLabel() vazio para conta sem id")
	}
}
