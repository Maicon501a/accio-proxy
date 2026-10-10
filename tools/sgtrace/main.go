// sgtrace: IAT + EAT hook tracer para revelar as fontes de fingerprint
// que o SecurityGuard SDK (Alibaba) lê durante initUmid/sign.
package main

/*
#cgo LDFLAGS: -lkernel32
#include <stdint.h>
#include <stdlib.h>
*/
import "C"

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	procVirtualProtect     = kernel32.NewProc("VirtualProtect")
	procVirtualAlloc       = kernel32.NewProc("VirtualAlloc")
	procGetCurrentProcess  = kernel32.NewProc("GetCurrentProcess")
	procGetModuleFileNameW = kernel32.NewProc("GetModuleFileNameW")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procGetProcAddress     = kernel32.NewProc("GetProcAddress")

	installMu    sync.Mutex
	installed    bool
	verboseFiles bool
	spoofEnabled bool
	spoofState   uint64 // PRNG state
)

func seedFrom(s string) uint64 {
	h := uint64(1469598103934665603)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	if h == 0 {
		h = 0x9E3779B97F4A7C15
	}
	return h
}

func nextRand() uint64 {
	spoofState ^= spoofState >> 12
	spoofState ^= spoofState << 25
	spoofState ^= spoofState >> 27
	return spoofState * 2685821657736338717
}

func randBytes(p unsafe.Pointer, n int) {
	for i := 0; i < n; i++ {
		v := nextRand()
		*(*byte)(unsafe.Pointer(uintptr(p) + uintptr(i))) = byte(v >> ((i % 8) * 8))
	}
}

// logger assíncrono com buffer grande (suporta funções quentes)
var (
	logMu  sync.Mutex
	logW   *bufio.Writer
	logF   *os.File
	logBuf chan string
)

func logger() {
	for s := range logBuf {
		logW.WriteString(s)
	}
}

func logf(format string, args ...interface{}) {
	if logBuf == nil {
		return
	}
	logBuf <- fmt.Sprintf("%d "+format+"\n", append([]interface{}{time.Now().UnixMilli()}, args...)...)
}

//export Flush
func Flush() {
	if logW != nil {
		logW.Flush()
	}
}

func gowstr(p *uint16) string {
	if p == nil {
		return "<nil>"
	}
	return syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(p))[:])
}

func gostr(p *byte) string {
	if p == nil {
		return "<nil>"
	}
	var b [4096]byte
	i := 0
	for ; i < len(b); i++ {
		c := *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i)))
		if c == 0 {
			break
		}
		b[i] = c
	}
	return string(b[:i])
}

func hasPrefix(s, p string) bool { return strings.HasPrefix(s, p) }
func contains(s, sub string) bool { return strings.Contains(s, sub) }
func lower(s string) string { return strings.ToLower(s) }

func callOrig(name string, args ...uintptr) uintptr {
	o, ok := orig[name]
	if !ok || o == 0 {
		return 0
	}
	r, _, _ := syscall.SyscallN(o, args...)
	return r
}

func derefU32(p *uint32) uint32 {
	if p == nil {
		return 0
	}
	return *p
}

// ---------- tabelas ----------

var hookedNames = map[string]bool{
	"GetVolumeInformationW": true, "GetComputerNameExW": true, "GetSystemInfo": true,
	"GetNativeSystemInfo": true, "GetDiskFreeSpaceExW": true, "GetLogicalDriveStringsW": true,
	"GetDriveTypeW": true, "GetSystemPowerStatus": true,
	"GetWindowsDirectoryW": true, "GetSystemWindowsDirectoryW": true,
	"GetUserNameW": true, "GetUserNameExW": true, "GetTokenInformation": true,
	"RegOpenKeyExW": true, "RegQueryValueExW": true, "RegGetValueW": true,
	"GetAdaptersAddresses": true, "GetAdaptersInfo": true, "GetIfTable2": true,
	"GetNetworkParams": true, "GetSystemMetrics": true, "SystemParametersInfoW": true,
	"CoCreateInstance": true, "CreateFileW": true, "DeviceIoControl": true,
	"GetSystemFirmwareTables": true, "EnumSystemFirmwareTables": true,
	"LoadLibraryW": true, "LoadLibraryExW": true, "GetProcAddress": true,
	"getaddrinfo": true, "gethostbyname": true, "connect": true,
	"NtQuerySystemInformation": true, "NtCreateFile": true,
	"NtOpenKey": true, "NtOpenKeyEx": true, "NtQueryValueKey": true,
	"RtlGetVersion": true,
}

func isHookedName(n string) bool { return hookedNames[n] }

// providers: DLLs onde a função pode ser resolvida (EAT)
var funcProviders = map[string][]string{
	"GetVolumeInformationW":      {"kernel32.dll", "kernelbase.dll"},
	"GetComputerNameExW":         {"kernel32.dll", "kernelbase.dll"},
	"GetSystemInfo":              {"kernel32.dll", "kernelbase.dll"},
	"GetNativeSystemInfo":        {"kernel32.dll", "kernelbase.dll"},
	"GetDiskFreeSpaceExW":        {"kernel32.dll", "kernelbase.dll"},
	"GetLogicalDriveStringsW":    {"kernel32.dll", "kernelbase.dll"},
	"GetDriveTypeW":              {"kernel32.dll", "kernelbase.dll"},
	"GetSystemPowerStatus":       {"kernel32.dll", "kernelbase.dll"},
	"GetWindowsDirectoryW":       {"kernel32.dll", "kernelbase.dll"},
	"GetSystemWindowsDirectoryW": {"kernel32.dll", "kernelbase.dll"},
	"CreateFileW":                {"kernel32.dll", "kernelbase.dll"},
	"DeviceIoControl":            {"kernel32.dll", "kernelbase.dll"},
	"GetSystemFirmwareTables":    {"kernel32.dll", "kernelbase.dll"},
	"EnumSystemFirmwareTables":   {"kernel32.dll", "kernelbase.dll"},
	"LoadLibraryW":               {"kernel32.dll", "kernelbase.dll"},
	"LoadLibraryExW":             {"kernel32.dll", "kernelbase.dll"},
	"GetProcAddress":             {"kernel32.dll", "kernelbase.dll"},
	"GetUserNameW":               {"advapi32.dll", "sechost.dll"},
	"GetTokenInformation":        {"advapi32.dll"},
	"RegOpenKeyExW":              {"advapi32.dll", "kernelbase.dll"},
	"RegQueryValueExW":           {"advapi32.dll", "kernelbase.dll"},
	"RegGetValueW":               {"advapi32.dll", "kernelbase.dll"},
	"GetAdaptersAddresses":       {"iphlpapi.dll"},
	"GetAdaptersInfo":            {"iphlpapi.dll"},
	"GetIfTable2":                {"iphlpapi.dll"},
	"GetNetworkParams":           {"iphlpapi.dll"},
	"GetSystemMetrics":           {"user32.dll"},
	"SystemParametersInfoW":      {"user32.dll"},
	"CoCreateInstance":           {"ole32.dll", "combase.dll"},
	"GetUserNameExW":             {"secur32.dll", "sspicli.dll"},
	"getaddrinfo":                {"ws2_32.dll"},
	"gethostbyname":              {"ws2_32.dll"},
	"connect":                    {"ws2_32.dll"},
	"NtQuerySystemInformation":   {"ntdll.dll"},
	"NtCreateFile":               {"ntdll.dll"},
	"NtOpenKey":                  {"ntdll.dll"},
	"NtOpenKeyEx":                {"ntdll.dll"},
	"NtQueryValueKey":            {"ntdll.dll"},
	"RtlGetVersion":              {"ntdll.dll"},
}

var (
	orig     = map[string]uintptr{} // nome -> endereço original resolvido
	hookAddr = map[string]uintptr{} // nome -> endereço do hook
)

// ---------- PE helpers ----------

func rvaToPtr(base uintptr, rva uint32) uintptr {
	nt := base + uintptr(*(*uint32)(unsafe.Pointer(base + 0x3C)))
	fileHeader := nt + 4
	optSize := *(*uint16)(unsafe.Pointer(fileHeader + 16))
	secCount := *(*uint16)(unsafe.Pointer(fileHeader + 2))
	opt := fileHeader + 20
	secs := opt + uintptr(optSize)
	for i := uint16(0); i < secCount; i++ {
		sh := secs + uintptr(i)*40
		va := *(*uint32)(unsafe.Pointer(sh + 12))
		vsz := *(*uint32)(unsafe.Pointer(sh + 8))
		rsz := *(*uint32)(unsafe.Pointer(sh + 16))
		sz := vsz
		if rsz > sz {
			sz = rsz
		}
		if rva >= va && rva < va+sz {
			return base + uintptr(rva)
		}
	}
	return 0
}

func moduleBaseName(base uintptr) string {
	var buf [512]uint16
	procGetModuleFileNameW.Call(uintptr(base), uintptr(unsafe.Pointer(&buf[0])), 512)
	full := syscall.UTF16ToString(buf[:])
	if i := strings.LastIndex(full, `\`); i >= 0 {
		return full[i+1:]
	}
	return full
}

// codecave por módulo: trampolines movabs rax, hook; jmp rax (12 bytes cada)
var moduleCaves = map[uintptr]uintptr{} // base -> próximo slot livre
var cavesMu     sync.Mutex

func caveAllocFor(base uintptr) uintptr {
	cavesMu.Lock()
	defer cavesMu.Unlock()
	if p, ok := moduleCaves[base]; ok && p != 0 {
		return p
	}
	// tenta alocar dentro de [base, base+0x7FFF0000) (RVA de 32 bits)
	var cave uintptr
	for off := uintptr(0x10000000); off < 0x7FFF0000; off += 0x1000000 {
		cand := (base + off) &^ 0xFFFF
		r, _, _ := procVirtualAlloc.Call(cand, 0x10000, 0x3000, 0x40) // MEM_RESERVE|MEM_COMMIT, PAGE_EXECUTE_READWRITE
		if r == cand {
			cave = r
			break
		}
		if r != 0 {
			VirtualFree(r)
		}
	}
	if cave == 0 {
		// fallback sem endereço preferido
		r, _, _ := procVirtualAlloc.Call(0, 0x10000, 0x3000, 0x40)
		if r == 0 || r < base || r-base >= 0x7FFF0000 {
			return 0
		}
		cave = r
	}
	moduleCaves[base] = cave
	return cave
}

func VirtualFree(p uintptr) {
	kernel32.NewProc("VirtualFree").Call(p, 0, 0x8000)
}

// escreve trampoline e devolve RVA dentro do módulo (0 = falha)
func writeTramp(moduleBase, hookAddrV uintptr) uint32 {
	cavesMu.Lock()
	slot := moduleCaves[moduleBase]
	cavesMu.Unlock()
	if slot == 0 {
		return 0
	}
	if slot+12 > (slot &^ 0xFFFF) + 0x10000 {
		return 0 // cave cheio (improvável: 30*12 = 360 bytes)
	}
	tramp := []byte{
		0x48, 0xB8, // movabs rax, imm64
		byte(hookAddrV), byte(hookAddrV >> 8), byte(hookAddrV >> 16), byte(hookAddrV >> 24),
		byte(hookAddrV >> 32), byte(hookAddrV >> 40), byte(hookAddrV >> 48), byte(hookAddrV >> 56),
		0xFF, 0xE0, // jmp rax
	}
	for i, b := range tramp {
		*(*byte)(unsafe.Pointer(slot + uintptr(i))) = b
	}
	cavesMu.Lock()
	moduleCaves[moduleBase] = slot + 12
	cavesMu.Unlock()
	return uint32(slot - moduleBase)
}

// patch EAT de um módulo: substitui endereços das funções alvo por trampolines locais
func patchEAT(base uintptr) int {
	bname := strings.ToLower(moduleBaseName(base))
	// só módulos provedores conhecidos
	isProvider := false
	for _, providers := range funcProviders {
		for _, p := range providers {
			if p == bname {
				isProvider = true
				break
			}
		}
		if isProvider {
			break
		}
	}
	if !isProvider {
		return 0
	}
	nt := base + uintptr(*(*uint32)(unsafe.Pointer(base + 0x3C)))
	fileHeader := nt + 4
	opt := fileHeader + 20
	magic := *(*uint16)(unsafe.Pointer(opt))
	var ddExport uint32
	if magic == 0x20B {
		ddExport = *(*uint32)(unsafe.Pointer(opt + 112))
	} else if magic == 0x10B {
		ddExport = *(*uint32)(unsafe.Pointer(opt + 96))
	} else {
		return 0
	}
	if ddExport == 0 {
		return 0
	}
	exp := rvaToPtr(base, ddExport)
	if exp == 0 {
		return 0
	}
	numNames := *(*uint32)(unsafe.Pointer(exp + 24))
	nameArr := rvaToPtr(base, *(*uint32)(unsafe.Pointer(exp+32)))
	ordArr := rvaToPtr(base, *(*uint32)(unsafe.Pointer(exp+36)))
	funcArr := rvaToPtr(base, *(*uint32)(unsafe.Pointer(exp+28)))
	count := 0
	for i := uint32(0); i < numNames; i++ {
		nrva := *(*uint32)(unsafe.Pointer(nameArr + uintptr(i)*4))
		nptr := rvaToPtr(base, nrva)
		if nptr == 0 {
			continue
		}
		nm := gostr((*byte)(unsafe.Pointer(nptr)))
		ord := *(*uint16)(unsafe.Pointer(ordArr + uintptr(i)*2))
		if !isHookedName(nm) {
			continue
		}
		ha := hookAddrOf(nm)
		if ha == 0 {
			continue
		}
		slot := funcArr + uintptr(ord)*4
		if hookAddr[nm] == 0 {
			// resolve original UMA vez via GPA (resolve forwarders)
			for _, prov := range funcProviders[nm] {
				pm, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(prov))))
				if pm == 0 {
					continue
				}
				np, _, _ := procGetProcAddress.Call(pm, uintptr(unsafe.Pointer(syscall.StringBytePtr(nm))))
				if np != 0 {
					orig[nm] = np
					break
				}
			}
			if orig[nm] == 0 {
				continue
			}
			hookAddr[nm] = ha
		}
		// trampoline local (EAT é RVA de 32 bits dentro do módulo)
		if _, hasCave := moduleCaves[base]; !hasCave {
			if caveAllocFor(base) == 0 {
				continue
			}
		}
		rva := writeTramp(base, ha)
		if rva == 0 {
			continue
		}
		var oldProt uint32
		procVirtualProtect.Call(slot, 4, 0x04, uintptr(unsafe.Pointer(&oldProt)))
		*(*uint32)(unsafe.Pointer(slot)) = rva
		procVirtualProtect.Call(slot, 4, uintptr(oldProt), uintptr(unsafe.Pointer(&oldProt)))
		count++
	}
	if count > 0 {
		logf("[eat] %s: %d exports hackeados", bname, count)
	}
	return count
}

// ---------- IAT patch (mantido para módulos já carregados) ----------

var patchedModules = map[uintptr]bool{}

func patchModule(base uintptr) {
	if base == 0 || patchedModules[base] {
		return
	}
	patchedModules[base] = true
	patchEAT(base)
	dos := *(*uint16)(unsafe.Pointer(base))
	if dos != 0x5A4D {
		return
	}
	ntOff := *(*uint32)(unsafe.Pointer(base + 0x3C))
	nt := base + uintptr(ntOff)
	if *(*uint32)(unsafe.Pointer(nt)) != 0x00004550 {
		return
	}
	fileHeader := nt + 4
	opt := fileHeader + 20
	magic := *(*uint16)(unsafe.Pointer(opt))
	var ddImport uint32
	if magic == 0x20B {
		ddImport = *(*uint32)(unsafe.Pointer(opt + 112 + 8))
	} else if magic == 0x10B {
		ddImport = *(*uint32)(unsafe.Pointer(opt + 96 + 8))
	} else {
		return
	}
	if ddImport == 0 {
		return
	}
	modName := moduleBaseName(base)
	count := 0
	desc := rvaToPtr(base, ddImport)
	for desc != 0 {
		oft := *(*uint32)(unsafe.Pointer(desc))
		nameRVA := *(*uint32)(unsafe.Pointer(desc + 12))
		ft := *(*uint32)(unsafe.Pointer(desc + 16))
		if oft == 0 && nameRVA == 0 && ft == 0 {
			break
		}
		dllNamePtr := rvaToPtr(base, nameRVA)
		if dllNamePtr != 0 {
			dllName := strings.ToLower(gostr((*byte)(unsafe.Pointer(dllNamePtr))))
			thunkRVA := oft
			if thunkRVA == 0 {
				thunkRVA = ft
			}
			thunk := rvaToPtr(base, thunkRVA)
			iat := rvaToPtr(base, ft)
			for idx := uintptr(0); ; idx++ {
				entry := *(*uintptr)(unsafe.Pointer(thunk + idx*8))
				if entry == 0 {
					break
				}
				if entry&0x8000000000000000 != 0 {
					continue
				}
				ibn := base + uintptr(entry)
				fname := gostr((*byte)(unsafe.Pointer(ibn + 2)))
				if !isHookedName(fname) {
					continue
				}
				// provider esperado?
				ok := false
				for _, p := range funcProviders[fname] {
					if p == dllName {
						ok = true
						break
					}
				}
				if !ok {
					continue
				}
				slot := iat + idx*8
				old := *(*uintptr)(unsafe.Pointer(slot))
				ha := hookAddrOf(fname)
				if ha == 0 || old == ha {
					continue
				}
				if hookAddr[fname] == 0 {
					hookAddr[fname] = ha
					orig[fname] = old
				}
				var oldProt uint32
				procVirtualProtect.Call(slot, 8, 0x04, uintptr(unsafe.Pointer(&oldProt)))
				*(*uintptr)(unsafe.Pointer(slot)) = ha
				procVirtualProtect.Call(slot, 8, uintptr(oldProt), uintptr(unsafe.Pointer(&oldProt)))
				count++
			}
		}
		desc += 20
	}
	if count > 0 {
		logf("[iat] %s: %d hooks", modName, count)
	}
}

var procEnumProcessModules = psapi.NewProc("EnumProcessModules")

func patchAllModules() {
	proc, _, _ := procGetCurrentProcess.Call()
	var mods [1024]uintptr
	var needed uint32
	r, _, _ := procEnumProcessModules.Call(proc, uintptr(unsafe.Pointer(&mods[0])), 1024*8, uintptr(unsafe.Pointer(&needed)))
	if r == 0 {
		logf("[patch] EnumProcessModules falhou")
		return
	}
	n := needed / 8
	if n > 1024 {
		n = 1024
	}
	for i := uint32(0); i < n; i++ {
		patchModule(mods[i])
	}
}

//export Install
func Install(logPath *C.char) C.int {
	installMu.Lock()
	defer installMu.Unlock()
	if installed {
		return 1
	}
	p := C.GoString(logPath)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return -1
	}
	logF = f
	logW = bufio.NewWriterSize(f, 1<<20)
	logBuf = make(chan string, 1<<16)
	go logger()
	if os.Getenv("SGTRACE_VERBOSE_FILES") == "1" {
		verboseFiles = true
	}
	if seed := os.Getenv("SGTRACE_SPOOF_SEED"); seed != "" {
		spoofEnabled = true
		spoofState = seedFrom(seed)
		logf("=== SPOOF MODE seed=%q ===", seed)
	}
	logf("=== sgtrace install (IAT+EAT) ===")
	patchAllModules()
	installed = true
	return 0
}

//export PatchNew
func PatchNew() C.int {
	patchAllModules()
	return 0
}

//export Uninstall
func Uninstall() C.int {
	logf("=== sgtrace fim ===")
	Flush()
	return 0
}

func main() {}
