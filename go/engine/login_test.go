package engine

import (
	"crypto/rand"
	"fmt"
	"os"
	"sync"
	"testing"

	"golang.org/x/net/context"

	"github.com/keybase/client/go/kex2"
	"github.com/keybase/client/go/libkb"
	keybase1 "github.com/keybase/client/go/protocol"
)

func TestLoginLogoutLogin(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := CreateAndSignupFakeUser(tc, "login")
	Logout(tc)
	u1.LoginOrBust(tc)
}

// Test login switching between two different users.
func TestLoginAndSwitch(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := CreateAndSignupFakeUser(tc, "first")
	Logout(tc)
	u2 := CreateAndSignupFakeUser(tc, "secon")
	Logout(tc)
	t.Logf("first logging back in")
	u1.LoginOrBust(tc)
	Logout(tc)
	t.Logf("second logging back in")
	u2.LoginOrBust(tc)

	return
}

func TestLoginFakeUserNoKeys(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	createFakeUserWithNoKeys(tc)

	me, err := libkb.LoadMe(libkb.NewLoadUserPubOptionalArg(tc.G))
	if err != nil {
		t.Fatal(err)
	}

	kf := me.GetKeyFamily()
	if kf == nil {
		t.Fatal("user has a nil key family")
	}
	if me.GetEldestKID().Exists() {
		t.Fatalf("user has an eldest key, they should have no keys: %s", me.GetEldestKID())
	}

	ckf := me.GetComputedKeyFamily()
	if ckf != nil {
		t.Errorf("user has a computed key family.  they shouldn't...")

		active := me.GetComputedKeyFamily().HasActiveKey()
		if active {
			t.Errorf("user has an active key, but they should have no keys")
		}
	}
}

func testUserHasDeviceKey(tc libkb.TestContext) {
	me, err := libkb.LoadMe(libkb.NewLoadUserPubOptionalArg(tc.G))
	if err != nil {
		tc.T.Fatal(err)
	}

	kf := me.GetKeyFamily()
	if kf == nil {
		tc.T.Fatal("user has a nil key family")
	}
	if me.GetEldestKID().IsNil() {
		tc.T.Fatal("user has no eldest key")
	}

	ckf := me.GetComputedKeyFamily()
	if ckf == nil {
		tc.T.Fatalf("user has no computed key family")
	}

	active := ckf.HasActiveKey()
	if !active {
		tc.T.Errorf("user has no active key")
	}

	subkey, err := me.GetDeviceSubkey()
	if err != nil {
		tc.T.Fatal(err)
	}
	if subkey == nil {
		tc.T.Fatal("nil subkey")
	}
}

// TestLoginInterrupt* tries to simulate what would happen if the
// locksmith login checkup gets interrupted.  See Issue #287.

// TestLoginInterruptDeviceRegister interrupts after registering a
// device and then tests that login corrects the situation on the
// next attempt.
/*
func TestLoginInterruptDeviceRegister(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	username, passphrase := createFakeUserWithNoKeys(tc)

	me, err := libkb.LoadMe(libkb.NewLoadUserPubOptionalArg(tc.G))
	if err != nil {
		t.Fatal(err)
	}
	secui := &libkb.TestSecretUI{Passphrase: passphrase}
	pps, err := tc.G.LoginState().GetPassphraseStream(secui)
	if err != nil {
		t.Fatal(err)
	}
	lks := libkb.NewLKSec(pps, me.GetUID(), tc.G)

	Logout(tc)

	// going to register a device only, not generating the device keys.
	dregArgs := &DeviceRegisterArgs{
		Me:   me,
		Name: "my new device",
		Lks:  lks,
	}
	dreg := NewDeviceRegister(dregArgs, tc.G)
	ctx := &Context{LogUI: tc.G.UI.GetLogUI(), LocksmithUI: &lockui{deviceName: "Device"}, GPGUI: &gpgtestui{}, SecretUI: secui, LoginUI: &libkb.TestLoginUI{}}
	if err := RunEngine(dreg, ctx); err != nil {
		t.Fatal(err)
	}

	// now login and see if it correctly generates needed keys
	li := NewLoginWithPassphraseEngine(username, passphrase, false, tc.G)
	if err := RunEngine(li, ctx); err != nil {
		t.Fatal(err)
	}
	if err := AssertLoggedIn(tc); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any keys, login should have fixed that:
	testUserHasDeviceKey(tc)
}

// TestLoginInterruptDevicePush interrupts before pushing device
// keys and then tests that login corrects the situation on the
// next attempt.
func TestLoginInterruptDevicePush(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	username, passphrase := createFakeUserWithNoKeys(tc)

	me, err := libkb.LoadMe(libkb.NewLoadUserPubOptionalArg(tc.G))
	if err != nil {
		t.Fatal(err)
	}
	secui := &libkb.TestSecretUI{Passphrase: passphrase}
	pps, err := tc.G.LoginState().GetPassphraseStream(secui)
	if err != nil {
		t.Fatal(err)
	}
	lks := libkb.NewLKSec(pps, me.GetUID(), tc.G)

	// going to register a device only, not generating the device keys.
	dregArgs := &DeviceRegisterArgs{
		Me:   me,
		Name: "my new device",
		Lks:  lks,
	}
	dreg := NewDeviceRegister(dregArgs, tc.G)
	ctx := &Context{LogUI: tc.G.UI.GetLogUI(), LocksmithUI: &lockui{deviceName: "Device"}, GPGUI: &gpgtestui{}, SecretUI: secui, LoginUI: &libkb.TestLoginUI{}}
	if err := RunEngine(dreg, ctx); err != nil {
		t.Fatal(err)
	}

	// now generate device keys but don't push them.
	dkeyArgs := &DeviceKeygenArgs{
		Me:         me,
		DeviceID:   dreg.DeviceID(),
		DeviceName: dregArgs.Name,
		DeviceType: libkb.DeviceTypeDesktop,
		Lks:        lks,
	}
	dkey := NewDeviceKeygen(dkeyArgs, tc.G)
	if err := RunEngine(dkey, ctx); err != nil {
		t.Fatal(err)
	}

	Logout(tc)

	// now login and see if it correctly generates needed keys
	li := NewLoginWithPassphraseEngine(username, passphrase, false, tc.G)
	if err := RunEngine(li, ctx); err != nil {
		t.Fatal(err)
	}
	if err := AssertLoggedIn(tc); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any keys, login should have fixed that:
	testUserHasDeviceKey(tc)
}

// TestLoginRestartProvision
//
// 1. Signup via the web and push a private pgp key
// 2. Login w/ go client.  Enter passphrase and get a session, but
//    don't complete the device provisioning.
// 3. Kill the daemon (`ctl stop`, reboot computer, etc.)
// 4. Login w/ go client again.  Should get logged out and ReloginRequiredError.
// 5. Login w/ go client.  Everything should work.
//
// This test should replicate that scenario.  While rare, we
// should make it work.
//
func TestLoginRestartProvision(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	// 1. Signup via the "web" and push a private pgp key
	u1 := createFakeUserWithPGPOnly(t, tc)
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	docui := &lockuiPGP{&lockui{deviceName: "PGP Device"}}

	// 2. Login w/ go client.  Enter passphrase and get a session, but
	//    don't complete the device provisioning.
	li := NewLoginWithPromptEngineSkipLocksmith(u1.Username, tc2.G)
	ctx := &Context{
		LogUI:       tc2.G.UI.GetLogUI(),
		LocksmithUI: docui,
		SecretUI:    u1.NewSecretUI(),
		GPGUI:       &gpgtestui{},
		LoginUI:     &libkb.TestLoginUI{},
	}
	if err := RunEngine(li, ctx); err != nil {
		t.Fatal(err)
	}

	// 3. Clearing the stream cache is the issue when daemon restarts.
	//    Doing a logout here would be too clean, and the next login would
	//    work fine.
	tc2.G.LoginState().Account(func(a *libkb.Account) {
		a.ClearStreamCache()
	}, "clear stream cache")

	// 4. Log in again
	li2 := NewLoginWithPromptEngine(u1.Username, tc2.G)
	ctx2 := &Context{
		LogUI:       tc2.G.UI.GetLogUI(),
		LocksmithUI: docui,
		SecretUI:    u1.NewSecretUI(),
		GPGUI:       &gpgtestui{},
		LoginUI:     &libkb.TestLoginUI{},
	}
	err := RunEngine(li2, ctx2)
	if err == nil {
		t.Fatal("expected relogin error, got nil")
	}
	if _, ok := err.(libkb.ReloginRequiredError); !ok {
		t.Fatalf("expected relogin error, got %T", err)
	}

	// login engine should logout the user before a relogin error.
	if err := AssertLoggedOut(tc2); err != nil {
		t.Fatal(err)
	}

	// this login attempt should finally work:
	li3 := NewLoginWithPromptEngine(u1.Username, tc2.G)
	ctx3 := &Context{
		LogUI:       tc2.G.UI.GetLogUI(),
		LocksmithUI: docui,
		SecretUI:    u1.NewSecretUI(),
		GPGUI:       &gpgtestui{},
		LoginUI:     &libkb.TestLoginUI{},
	}
	if err := RunEngine(li3, ctx3); err != nil {
		t.Fatal(err)
	}

	// and they should have their device keys and paper device
	testUserHasDeviceKey(tc2)
	hasOnePaperDev(tc2, u1)
}
*/

func TestUserInfo(t *testing.T) {
	t.Skip()
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u := CreateAndSignupFakeUser(tc, "login")
	var username libkb.NormalizedUsername
	var err error
	aerr := tc.G.LoginState().Account(func(a *libkb.Account) {
		_, username, _, _, err = a.UserInfo()
	}, "TestUserInfo")
	if aerr != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !username.Eq(libkb.NewNormalizedUsername(u.Username)) {
		t.Errorf("userinfo username: %q, expected %q", username, u.Username)
	}
}

func TestLogin(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "kex2provision")
	defer tcX.Cleanup()

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "template")
	defer tcY.Cleanup()

	// provisioner needs to be logged in
	userX := CreateAndSignupFakeUser(tcX, "login")
	var secretX kex2.Secret
	if _, err := rand.Read(secretX[:]); err != nil {
		t.Fatal(err)
	}

	secretCh := make(chan kex2.Secret)

	// provisionee calls xlogin:
	ctx := &Context{
		ProvisionUI: newTestProvisionUISecretCh(secretCh),
		LoginUI:     &libkb.TestLoginUI{},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "")

	var wg sync.WaitGroup

	// start provisionee
	wg.Add(1)
	go func() {
		defer wg.Done()
		tcY.G.Log.Warning("starting xlogin")
		if err := RunEngine(eng, ctx); err != nil {
			t.Errorf("xlogin error: %s", err)
			return
		}
	}()

	// start provisioner
	provisioner := NewKex2Provisioner(tcX.G, secretX, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx := &Context{
			SecretUI:    userX.NewSecretUI(),
			ProvisionUI: newTestProvisionUI(),
		}
		if err := RunEngine(provisioner, ctx); err != nil {
			t.Errorf("provisioner error: %s", err)
			return
		}
	}()
	secretFromY := <-secretCh
	provisioner.AddSecret(secretFromY)

	wg.Wait()

	if err := AssertProvisioned(tcY); err != nil {
		t.Fatal(err)
	}
}

// If a user has device keys, selecting the username/passphrase
// provisioning option should fail.
func TestProvisionPassphraseFail(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "provision_x")
	defer tcX.Cleanup()

	// create user (and device X)
	userX := CreateAndSignupFakeUser(tcX, "login")

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "provision_y")
	defer tcY.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: userX.Username},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "")
	err := RunEngine(eng, ctx)
	if err == nil {
		t.Fatal("expected xlogin to fail, but it ran without error")
	}
	if _, ok := err.(libkb.PassphraseProvisionImpossibleError); !ok {
		t.Fatalf("expected PassphraseProvisionImpossibleError, got %T (%s)", err, err)
	}

	if err := AssertLoggedIn(tcY); err == nil {
		t.Fatal("should not be logged in")
	}
}

// If a user has no keys, provision via passphrase should work.
func TestProvisionPassphraseNoKeys(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	username, passphrase := createFakeUserWithNoKeys(tc)

	Logout(tc)

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{Passphrase: passphrase},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "")
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any keys, login should have fixed that:
	testUserHasDeviceKey(tc)

	// and they should have a paper backup key
	hasOnePaperDev(tc, &FakeUser{Username: username, Passphrase: passphrase})

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

// If a user has (only) a synced pgp key, provision via passphrase
// should work.
func TestProvisionPassphraseSyncedPGP(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	u1 := createFakeUserWithPGPOnly(t, tc)
	Logout(tc)
	tc.Cleanup()

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc = SetupEngineTest(t, "login")
	defer tc.Cleanup()

	ctx := &Context{
		ProvisionUI: newTestProvisionUIPassphrase(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		LogUI:       tc.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc.G, libkb.DeviceTypeDesktop, "")
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	// since this user didn't have any device keys, xlogin should have fixed that:
	testUserHasDeviceKey(tc)

	// and they should have a paper backup key
	hasOnePaperDev(tc, u1)

	if err := AssertProvisioned(tc); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionPaper(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()
	fu := NewFakeUserOrBust(t, "paper")
	arg := MakeTestSignupEngineRunArg(fu)
	loginUI := &paperLoginUI{Username: fu.Username}
	ctx := &Context{
		LogUI:    tc.G.UI.GetLogUI(),
		GPGUI:    &gpgtestui{},
		SecretUI: fu.NewSecretUI(),
		LoginUI:  loginUI,
	}
	s := NewSignupEngine(&arg, tc.G)
	err := RunEngine(s, ctx)
	if err != nil {
		tc.T.Fatal(err)
	}

	assertNumDevicesAndKeys(tc, fu, 2, 4)

	Logout(tc)

	if len(loginUI.PaperPhrase) == 0 {
		t.Fatal("login ui has no paper key phrase")
	}

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	secui := fu.NewSecretUI()
	secui.BackupPassphrase = loginUI.PaperPhrase

	ctx = &Context{
		ProvisionUI: newTestProvisionUIPaper(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    secui,
		LoginUI:     &libkb.TestLoginUI{Username: fu.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "")
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	assertNumDevicesAndKeys(tc, fu, 3, 6)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}
}

// Provision device using a private GPG key (not synced to keybase
// server).
func TestProvisionGPG(t *testing.T) {
	tc := SetupEngineTest(t, "login")
	defer tc.Cleanup()

	u1 := createFakeUserWithPGPPubOnly(t, tc)
	Logout(tc)

	// redo SetupEngineTest to get a new home directory...should look like a new device.
	tc2 := SetupEngineTest(t, "login")
	defer tc2.Cleanup()

	// we need the gpg keyring that's in the first homedir
	if err := tc.MoveGpgKeyringTo(tc2); err != nil {
		t.Fatal(err)
	}

	// now safe to cleanup first home
	tc.Cleanup()

	// run xlogin on new device
	ctx := &Context{
		ProvisionUI: newTestProvisionUIGPG(),
		LogUI:       tc2.G.UI.GetLogUI(),
		SecretUI:    u1.NewSecretUI(),
		LoginUI:     &libkb.TestLoginUI{Username: u1.Username},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tc2.G, libkb.DeviceTypeDesktop, "")
	if err := RunEngine(eng, ctx); err != nil {
		t.Fatal(err)
	}

	testUserHasDeviceKey(tc2)

	// highly possible they didn't have a paper key, so make sure they have one now:
	hasOnePaperDev(tc2, u1)

	if err := AssertProvisioned(tc2); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionDupDevice(t *testing.T) {
	// device X (provisioner) context:
	tcX := SetupEngineTest(t, "kex2provision")
	defer tcX.Cleanup()

	// device Y (provisionee) context:
	tcY := SetupEngineTest(t, "template")
	defer tcY.Cleanup()

	// provisioner needs to be logged in
	userX := CreateAndSignupFakeUser(tcX, "login")
	var secretX kex2.Secret
	if _, err := rand.Read(secretX[:]); err != nil {
		t.Fatal(err)
	}

	secretCh := make(chan kex2.Secret)

	provui := &testProvisionDupDeviceUI{newTestProvisionUISecretCh(secretCh)}

	// provisionee calls xlogin:
	ctx := &Context{
		ProvisionUI: provui,
		LoginUI:     &libkb.TestLoginUI{},
		LogUI:       tcY.G.UI.GetLogUI(),
		SecretUI:    &libkb.TestSecretUI{},
		GPGUI:       &gpgtestui{},
	}
	eng := NewLogin(tcY.G, libkb.DeviceTypeDesktop, "")

	var wg sync.WaitGroup

	// start provisionee
	wg.Add(1)
	go func() {
		defer wg.Done()
		tcY.G.Log.Warning("starting xlogin")
		if err := RunEngine(eng, ctx); err == nil {
			t.Errorf("xlogin ran without error")
			return
		}
	}()

	// start provisioner
	provisioner := NewKex2Provisioner(tcX.G, secretX, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx := &Context{
			SecretUI:    userX.NewSecretUI(),
			ProvisionUI: newTestProvisionUI(),
		}
		if err := RunEngine(provisioner, ctx); err == nil {
			t.Errorf("provisioner ran without error")
			return
		}
	}()
	secretFromY := <-secretCh
	provisioner.AddSecret(secretFromY)

	wg.Wait()

	if err := AssertProvisioned(tcY); err == nil {
		t.Fatal("device provisioned using existing name")
	}
}

type testProvisionUI struct {
	secretCh chan kex2.Secret
	method   keybase1.ProvisionMethod
	verbose  bool
}

func newTestProvisionUI() *testProvisionUI {
	ui := &testProvisionUI{method: keybase1.ProvisionMethod_DEVICE}
	if len(os.Getenv("KB_TEST_VERBOSE")) > 0 {
		ui.verbose = true
	}
	return ui
}

func newTestProvisionUISecretCh(ch chan kex2.Secret) *testProvisionUI {
	ui := newTestProvisionUI()
	ui.secretCh = ch
	return ui
}

func newTestProvisionUIPassphrase() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_PASSPHRASE
	return ui
}

func newTestProvisionUIPaper() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_PAPER_KEY
	return ui
}

func newTestProvisionUIGPG() *testProvisionUI {
	ui := newTestProvisionUI()
	ui.method = keybase1.ProvisionMethod_GPG
	return ui
}

func (u *testProvisionUI) printf(format string, a ...interface{}) {
	if !u.verbose {
		return
	}
	fmt.Printf("testProvisionUI: "+format+"\n", a...)
}

func (u *testProvisionUI) ChooseProvisioningMethod(_ context.Context, _ keybase1.ChooseProvisioningMethodArg) (keybase1.ProvisionMethod, error) {
	u.printf("ChooseProvisioningMethod")
	return u.method, nil
}

func (u *testProvisionUI) ChooseDeviceType(_ context.Context, _ int) (keybase1.DeviceType, error) {
	u.printf("ChooseProvisionerDevice")
	return keybase1.DeviceType_DESKTOP, nil
}

func (u *testProvisionUI) DisplayAndPromptSecret(_ context.Context, arg keybase1.DisplayAndPromptSecretArg) ([]byte, error) {
	u.printf("DisplayAndPromptSecret")
	var ks kex2.Secret
	copy(ks[:], arg.Secret)
	u.secretCh <- ks
	return nil, nil
}

func (u *testProvisionUI) PromptNewDeviceName(_ context.Context, arg keybase1.PromptNewDeviceNameArg) (string, error) {
	u.printf("PromptNewDeviceName")
	return "device_xxx", nil
}

func (u *testProvisionUI) DisplaySecretExchanged(_ context.Context, _ int) error {
	u.printf("DisplaySecretExchanged")
	return nil
}

func (u *testProvisionUI) ProvisioneeSuccess(_ context.Context, _ keybase1.ProvisioneeSuccessArg) error {
	u.printf("ProvisioneeSuccess")
	return nil
}

func (u *testProvisionUI) ProvisionerSuccess(_ context.Context, _ keybase1.ProvisionerSuccessArg) error {
	u.printf("ProvisionerSuccess")
	return nil
}

type testProvisionDupDeviceUI struct {
	*testProvisionUI
}

// return an existing device name
func (u *testProvisionDupDeviceUI) PromptNewDeviceName(_ context.Context, arg keybase1.PromptNewDeviceNameArg) (string, error) {
	return arg.ExistingDevices[0], nil
}

type paperLoginUI struct {
	Username    string
	PaperPhrase string
}

func (p *paperLoginUI) GetEmailOrUsername(_ context.Context, _ int) (string, error) {
	return p.Username, nil
}

func (p *paperLoginUI) PromptRevokePaperKeys(_ context.Context, arg keybase1.PromptRevokePaperKeysArg) (bool, error) {
	return false, nil
}

func (p *paperLoginUI) DisplayPaperKeyPhrase(_ context.Context, arg keybase1.DisplayPaperKeyPhraseArg) error {
	return nil
}

func (p *paperLoginUI) DisplayPrimaryPaperKey(_ context.Context, arg keybase1.DisplayPrimaryPaperKeyArg) error {
	p.PaperPhrase = arg.Phrase
	return nil
}
