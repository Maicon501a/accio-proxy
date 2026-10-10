// hooks: todas as funções hook exportadas + helpers de endereço.
// Cada hook loga e encaminha para o original salvo em orig[name].
package main

/*
#cgo LDFLAGS: -lkernel32
#include <stdint.h>
#include <stdlib.h>

extern int HookGetVolumeInformationW(unsigned short*, unsigned short*, unsigned int, unsigned int*, unsigned int*, unsigned int*, unsigned short*, unsigned int);
extern int HookGetComputerNameExW(unsigned int, unsigned short*, unsigned int*);
extern void HookGetNativeSystemInfo(void*);
extern void HookGetSystemInfo(void*);
extern unsigned int HookGetDiskFreeSpaceExW(unsigned short*, unsigned long long*, unsigned long long*, unsigned long long*);
extern unsigned int HookGetLogicalDriveStringsW(unsigned int, unsigned short*);
extern unsigned int HookGetDriveTypeW(unsigned short*);
extern int HookGetSystemPowerStatus(void*);
extern unsigned int HookGetWindowsDirectoryW(unsigned short*, unsigned int);
extern unsigned int HookGetSystemWindowsDirectoryW(unsigned short*, unsigned int);
extern int HookGetUserNameW(unsigned short*, unsigned int*);
extern int HookGetUserNameExW(unsigned int, unsigned short*, unsigned int*);
extern int HookGetTokenInformation(unsigned long long, unsigned int, void*, unsigned int, unsigned int*);
extern unsigned long long HookRegOpenKeyExW(unsigned long long, unsigned short*, unsigned int, unsigned int, unsigned long long*);
extern unsigned long long HookRegQueryValueExW(unsigned long long, unsigned short*, unsigned int*, unsigned int*, void*, unsigned int*);
extern unsigned long long HookRegGetValueW(unsigned long long, unsigned short*, unsigned short*, unsigned int, unsigned int*, void*, unsigned int*);
extern unsigned int HookGetAdaptersAddresses(unsigned int, unsigned int, void*, void*, unsigned int*);
extern unsigned int HookGetAdaptersInfo(void*, unsigned int*);
extern unsigned int HookGetIfTable2(void**, void*);
extern unsigned int HookGetNetworkParams(void*, unsigned int*);
extern int HookGetSystemMetrics(int);
extern int HookSystemParametersInfoW(unsigned int, unsigned int, void*, unsigned int);
extern unsigned long long HookCoCreateInstance(void*, void*, unsigned int, void*, void**);
extern unsigned long long HookCreateFileW(unsigned short*, unsigned int, unsigned int, void*, unsigned int, unsigned int, unsigned long long);
extern int HookDeviceIoControl(unsigned long long, unsigned int, void*, unsigned int, void*, unsigned int, unsigned int*, void*);
extern unsigned int HookGetSystemFirmwareTables(unsigned int, void*, unsigned int);
extern unsigned int HookEnumSystemFirmwareTables(unsigned int, void*, unsigned int);
extern unsigned long long HookLoadLibraryExW(unsigned short*, void*, unsigned int);
extern unsigned long long HookLoadLibraryW(unsigned short*);
extern unsigned long long HookGetProcAddress(unsigned long long, unsigned char*);
extern int HookGetAddrInfo(unsigned short*, unsigned short*, void*, void**);
extern void* HookGetHostByName(unsigned char*);
extern int HookConnect(unsigned long long, void*, int);
extern unsigned long long HookNtQuerySystemInformation(unsigned int, void*, unsigned int, unsigned int*);
extern unsigned long long HookNtCreateFile(unsigned long long**, unsigned int, void*, void*, void*, unsigned int, unsigned int, unsigned int, unsigned int, void*, unsigned int);
extern unsigned long long HookNtOpenKey(unsigned long long**, unsigned int, void*);
extern unsigned long long HookNtOpenKeyEx(unsigned long long**, unsigned int, void*, unsigned int);
extern unsigned long long HookNtQueryValueKey(unsigned long long, void*, unsigned int, void*, unsigned int, unsigned int*);
extern unsigned long long HookRtlGetVersion(void*);

static uintptr_t A_GVW(){return (uintptr_t)&HookGetVolumeInformationW;}
static uintptr_t A_CNE(){return (uintptr_t)&HookGetComputerNameExW;}
static uintptr_t A_GNSI(){return (uintptr_t)&HookGetNativeSystemInfo;}
static uintptr_t A_GSI(){return (uintptr_t)&HookGetSystemInfo;}
static uintptr_t A_GDFS(){return (uintptr_t)&HookGetDiskFreeSpaceExW;}
static uintptr_t A_GLDS(){return (uintptr_t)&HookGetLogicalDriveStringsW;}
static uintptr_t A_GDT(){return (uintptr_t)&HookGetDriveTypeW;}
static uintptr_t A_GPS(){return (uintptr_t)&HookGetSystemPowerStatus;}
static uintptr_t A_GWD(){return (uintptr_t)&HookGetWindowsDirectoryW;}
static uintptr_t A_GSWD(){return (uintptr_t)&HookGetSystemWindowsDirectoryW;}
static uintptr_t A_GUN(){return (uintptr_t)&HookGetUserNameW;}
static uintptr_t A_GUNX(){return (uintptr_t)&HookGetUserNameExW;}
static uintptr_t A_GTI(){return (uintptr_t)&HookGetTokenInformation;}
static uintptr_t A_ROK(){return (uintptr_t)&HookRegOpenKeyExW;}
static uintptr_t A_RQV(){return (uintptr_t)&HookRegQueryValueExW;}
static uintptr_t A_RGV(){return (uintptr_t)&HookRegGetValueW;}
static uintptr_t A_GAA(){return (uintptr_t)&HookGetAdaptersAddresses;}
static uintptr_t A_GAI(){return (uintptr_t)&HookGetAdaptersInfo;}
static uintptr_t A_GIT2(){return (uintptr_t)&HookGetIfTable2;}
static uintptr_t A_GNP(){return (uintptr_t)&HookGetNetworkParams;}
static uintptr_t A_GSM(){return (uintptr_t)&HookGetSystemMetrics;}
static uintptr_t A_SPI(){return (uintptr_t)&HookSystemParametersInfoW;}
static uintptr_t A_COI(){return (uintptr_t)&HookCoCreateInstance;}
static uintptr_t A_CFW(){return (uintptr_t)&HookCreateFileW;}
static uintptr_t A_DIO(){return (uintptr_t)&HookDeviceIoControl;}
static uintptr_t A_GSFT(){return (uintptr_t)&HookGetSystemFirmwareTables;}
static uintptr_t A_ESFT(){return (uintptr_t)&HookEnumSystemFirmwareTables;}
static uintptr_t A_LLEX(){return (uintptr_t)&HookLoadLibraryExW;}
static uintptr_t A_LLW(){return (uintptr_t)&HookLoadLibraryW;}
static uintptr_t A_GPA(){return (uintptr_t)&HookGetProcAddress;}
static uintptr_t A_GAI_ADDR(){return (uintptr_t)&HookGetAddrInfo;}
static uintptr_t A_GHBN(){return (uintptr_t)&HookGetHostByName;}
static uintptr_t A_CONN(){return (uintptr_t)&HookConnect;}
static uintptr_t A_NQSI(){return (uintptr_t)&HookNtQuerySystemInformation;}
static uintptr_t A_NCF(){return (uintptr_t)&HookNtCreateFile;}
static uintptr_t A_NOK(){return (uintptr_t)&HookNtOpenKey;}
static uintptr_t A_NOKE(){return (uintptr_t)&HookNtOpenKeyEx;}
static uintptr_t A_NQVK(){return (uintptr_t)&HookNtQueryValueKey;}
static uintptr_t A_RGV2(){return (uintptr_t)&HookRtlGetVersion;}
*/
import "C"

import (
	"os"
	"syscall"
	"unsafe"
)

func hookAddrOf(name string) uintptr {
	switch name {
	case "GetVolumeInformationW":
		return uintptr(C.A_GVW())
	case "GetComputerNameExW":
		return uintptr(C.A_CNE())
	case "GetNativeSystemInfo":
		return uintptr(C.A_GNSI())
	case "GetSystemInfo":
		return uintptr(C.A_GSI())
	case "GetDiskFreeSpaceExW":
		return uintptr(C.A_GDFS())
	case "GetLogicalDriveStringsW":
		return uintptr(C.A_GLDS())
	case "GetDriveTypeW":
		return uintptr(C.A_GDT())
	case "GetSystemPowerStatus":
		return uintptr(C.A_GPS())
	case "GetWindowsDirectoryW":
		return uintptr(C.A_GWD())
	case "GetSystemWindowsDirectoryW":
		return uintptr(C.A_GSWD())
	case "GetUserNameW":
		return uintptr(C.A_GUN())
	case "GetUserNameExW":
		return uintptr(C.A_GUNX())
	case "GetTokenInformation":
		return uintptr(C.A_GTI())
	case "RegOpenKeyExW":
		return uintptr(C.A_ROK())
	case "RegQueryValueExW":
		return uintptr(C.A_RQV())
	case "RegGetValueW":
		return uintptr(C.A_RGV())
	case "GetAdaptersAddresses":
		return uintptr(C.A_GAA())
	case "GetAdaptersInfo":
		return uintptr(C.A_GAI())
	case "GetIfTable2":
		return uintptr(C.A_GIT2())
	case "GetNetworkParams":
		return uintptr(C.A_GNP())
	case "GetSystemMetrics":
		return uintptr(C.A_GSM())
	case "SystemParametersInfoW":
		return uintptr(C.A_SPI())
	case "CoCreateInstance":
		return uintptr(C.A_COI())
	case "CreateFileW":
		return uintptr(C.A_CFW())
	case "DeviceIoControl":
		return uintptr(C.A_DIO())
	case "GetSystemFirmwareTables":
		return uintptr(C.A_GSFT())
	case "EnumSystemFirmwareTables":
		return uintptr(C.A_ESFT())
	case "LoadLibraryExW":
		return uintptr(C.A_LLEX())
	case "LoadLibraryW":
		return uintptr(C.A_LLW())
	case "GetProcAddress":
		return uintptr(C.A_GPA())
	case "getaddrinfo":
		return uintptr(C.A_GAI_ADDR())
	case "gethostbyname":
		return uintptr(C.A_GHBN())
	case "connect":
		return uintptr(C.A_CONN())
	case "NtQuerySystemInformation":
		return uintptr(C.A_NQSI())
	case "NtCreateFile":
		return uintptr(C.A_NCF())
	case "NtOpenKey":
		return uintptr(C.A_NOK())
	case "NtOpenKeyEx":
		return uintptr(C.A_NOKE())
	case "NtQueryValueKey":
		return uintptr(C.A_NQVK())
	case "RtlGetVersion":
		return uintptr(C.A_RGV2())
	}
	return 0
}

// ---------------- hooks ----------------

//export HookGetVolumeInformationW
func HookGetVolumeInformationW(root *uint16, volName *uint16, nameSize uint32, serial *uint32, maxLen *uint32, flags *uint32, fsName *uint16, fsSize uint32) int32 {
	logf("[GVW] root=%s", gowstr(root))
	r := callOrig("GetVolumeInformationW",
		uintptr(unsafe.Pointer(root)), uintptr(unsafe.Pointer(volName)), uintptr(nameSize),
		uintptr(unsafe.Pointer(serial)), uintptr(unsafe.Pointer(maxLen)), uintptr(unsafe.Pointer(flags)),
		uintptr(unsafe.Pointer(fsName)), uintptr(fsSize))
	if serial != nil {
		logf("[GVW]   -> serial=%08x", *serial)
		if spoofEnabled && r != 0 {
			*serial = uint32(nextRand())
			logf("[SPOOF] GVW serial -> %08x", *serial)
		}
	}
	return int32(r)
}

//export HookGetComputerNameExW
func HookGetComputerNameExW(nameType uint32, buf *uint16, size *uint32) int32 {
	r := callOrig("GetComputerNameExW", uintptr(nameType), uintptr(unsafe.Pointer(buf)), uintptr(unsafe.Pointer(size)))
	logf("[ComputerNameEx] type=%d -> %s", nameType, gowstr(buf))
	return int32(r)
}

//export HookGetSystemInfo
func HookGetSystemInfo(si unsafe.Pointer) {
	logf("[GetSystemInfo]")
	callOrig("GetSystemInfo", uintptr(si))
	if spoofEnabled && si != nil {
		// SYSTEM_INFO x64: wProcessorLevel @44, wProcessorRevision @46
		randBytes(unsafe.Pointer(uintptr(si)+44), 4)
		logf("[SPOOF] GetSystemInfo level/revision")
	}
}

//export HookGetNativeSystemInfo
func HookGetNativeSystemInfo(si unsafe.Pointer) {
	logf("[GetNativeSystemInfo]")
	callOrig("GetNativeSystemInfo", uintptr(si))
	if spoofEnabled && si != nil {
		randBytes(unsafe.Pointer(uintptr(si)+44), 4)
		logf("[SPOOF] NativeSystemInfo level/revision")
	}
}

//export HookGetDiskFreeSpaceExW
func HookGetDiskFreeSpaceExW(dir *uint16, free *uint64, total *uint64, totalFree *uint64) uint32 {
	r := callOrig("GetDiskFreeSpaceExW", uintptr(unsafe.Pointer(dir)), uintptr(unsafe.Pointer(free)), uintptr(unsafe.Pointer(total)), uintptr(unsafe.Pointer(totalFree)))
	logf("[DiskFreeSpace] dir=%s", gowstr(dir))
	return uint32(r)
}

//export HookGetLogicalDriveStringsW
func HookGetLogicalDriveStringsW(size uint32, buf *uint16) uint32 {
	r := callOrig("GetLogicalDriveStringsW", uintptr(size), uintptr(unsafe.Pointer(buf)))
	logf("[LogicalDriveStrings]")
	return uint32(r)
}

//export HookGetDriveTypeW
func HookGetDriveTypeW(root *uint16) uint32 {
	r := callOrig("GetDriveTypeW", uintptr(unsafe.Pointer(root)))
	logf("[DriveType] root=%s -> %d", gowstr(root), uint32(r))
	return uint32(r)
}

//export HookGetSystemPowerStatus
func HookGetSystemPowerStatus(sps unsafe.Pointer) int32 {
	logf("[SystemPowerStatus]")
	return int32(callOrig("GetSystemPowerStatus", uintptr(sps)))
}

//export HookGetWindowsDirectoryW
func HookGetWindowsDirectoryW(buf *uint16, size uint32) uint32 {
	r := callOrig("GetWindowsDirectoryW", uintptr(unsafe.Pointer(buf)), uintptr(size))
	logf("[WinDir] -> %s", gowstr(buf))
	return uint32(r)
}

//export HookGetSystemWindowsDirectoryW
func HookGetSystemWindowsDirectoryW(buf *uint16, size uint32) uint32 {
	r := callOrig("GetSystemWindowsDirectoryW", uintptr(unsafe.Pointer(buf)), uintptr(size))
	logf("[SysWinDir] -> %s", gowstr(buf))
	return uint32(r)
}

//export HookGetUserNameW
func HookGetUserNameW(buf *uint16, size *uint32) int32 {
	r := callOrig("GetUserNameW", uintptr(unsafe.Pointer(buf)), uintptr(unsafe.Pointer(size)))
	logf("[GetUserName] -> %s", gowstr(buf))
	return int32(r)
}

//export HookGetUserNameExW
func HookGetUserNameExW(nameFormat uint32, buf *uint16, size *uint32) int32 {
	r := callOrig("GetUserNameExW", uintptr(nameFormat), uintptr(unsafe.Pointer(buf)), uintptr(unsafe.Pointer(size)))
	logf("[GetUserNameEx] fmt=%d -> %s", nameFormat, gowstr(buf))
	return int32(r)
}

//export HookGetTokenInformation
func HookGetTokenInformation(tok uintptr, class uint32, info unsafe.Pointer, length uint32, ret *uint32) int32 {
	r := callOrig("GetTokenInformation", tok, uintptr(class), uintptr(info), uintptr(length), uintptr(unsafe.Pointer(ret)))
	logf("[GetTokenInformation] class=%d", class)
	return int32(r)
}

//export HookRegOpenKeyExW
func HookRegOpenKeyExW(hKey uintptr, subKey *uint16, opts uint32, sam uint32, out *uintptr) uintptr {
	r := callOrig("RegOpenKeyExW", hKey, uintptr(unsafe.Pointer(subKey)), uintptr(opts), uintptr(sam), uintptr(unsafe.Pointer(out)))
	logf("[RegOpenKeyExW] hk=%x sub=%s sam=%x -> r=%x", hKey, gowstr(subKey), sam, r)
	return r
}

//export HookRegQueryValueExW
func HookRegQueryValueExW(hKey uintptr, val *uint16, reserved *uint32, typ *uint32, data unsafe.Pointer, size *uint32) uintptr {
	r := callOrig("RegQueryValueExW", hKey, uintptr(unsafe.Pointer(val)), uintptr(unsafe.Pointer(reserved)),
		uintptr(unsafe.Pointer(typ)), uintptr(data), uintptr(unsafe.Pointer(size)))
	vs := gowstr(val)
	logf("[RegQueryValueExW] h=%x val=%s -> r=%x (type=%d size=%d)", hKey, vs, r, derefU32(typ), derefU32(size))
	if spoofEnabled && r == 0 && vs == "InstallTime" && data != nil && size != nil && *size >= 8 {
		randBytes(data, 8)
		logf("[SPOOF] InstallTime -> %x", *(*uint64)(data))
	}
	return r
}

//export HookRegGetValueW
func HookRegGetValueW(hKey uintptr, subKey *uint16, val *uint16, flags uint32, typ *uint32, data unsafe.Pointer, size *uint32) uintptr {
	r := callOrig("RegGetValueW", hKey, uintptr(unsafe.Pointer(subKey)), uintptr(unsafe.Pointer(val)), uintptr(flags),
		uintptr(unsafe.Pointer(typ)), uintptr(data), uintptr(unsafe.Pointer(size)))
	logf("[RegGetValueW] hk=%x sub=%s val=%s -> r=%x", hKey, gowstr(subKey), gowstr(val), r)
	return r
}

//export HookGetAdaptersAddresses
func HookGetAdaptersAddresses(family uint32, flags uint32, reserved unsafe.Pointer, buf unsafe.Pointer, size *uint32) uint32 {
	r := callOrig("GetAdaptersAddresses", uintptr(family), uintptr(flags), uintptr(reserved), uintptr(buf), uintptr(unsafe.Pointer(size)))
	logf("[GetAdaptersAddresses] fam=%d flags=%x -> r=%x", family, flags, r)
	if spoofEnabled && r == 0 && buf != nil && size != nil {
		// IP_ADAPTER_ADDRESSES x64: Next@8, PhysLen@80, PhysAddr@81
		start := uintptr(buf)
		end := start + uintptr(*size)
		p := start
		for i := 0; i < 64; i++ {
			if p == 0 || p < start || p+96 > end {
				break
			}
			next := *(*uintptr)(unsafe.Pointer(p + 8))
			pl := *(*byte)(unsafe.Pointer(p + 80))
			if pl > 0 && pl <= 32 {
				spoofMAC(p+81, int(pl))
				logf("[SPOOF] GAA mac len=%d", pl)
			}
			if next == 0 || next < start || next >= end {
				break
			}
			p = next
		}
	}
	return uint32(r)
}

//export HookGetAdaptersInfo
func HookGetAdaptersInfo(buf unsafe.Pointer, size *uint32) uint32 {
	r := callOrig("GetAdaptersInfo", uintptr(buf), uintptr(unsafe.Pointer(size)))
	logf("[GetAdaptersInfo] -> r=%x", r)
	if spoofEnabled && r == 0 && buf != nil && size != nil {
		// IP_ADAPTER_INFO: Next@0, AddressLength@404, Address@408 (ASCII desc @272 opcional)
		start := uintptr(buf)
		end := start + uintptr(*size)
		p := start
		for i := 0; i < 64; i++ {
			if p == 0 || p < start || p+416 > end {
				break
			}
			next := *(*uintptr)(unsafe.Pointer(p))
			al := *(*uint32)(unsafe.Pointer(p + 404))
			if al > 0 && al <= 8 {
				spoofMAC(p+408, int(al))
				logf("[SPOOF] GAI mac len=%d", al)
			}
			if next == 0 || next < start || next >= end {
				break
			}
			p = next
		}
	}
	return uint32(r)
}

// spoofMAC randomiza a cauda do MAC (mantém OUI=3 primeiros bytes, força
// bit unicast) para parecer um NIC real do mesmo fabricante.
func spoofMAC(p uintptr, n int) {
	if n < 4 {
		randBytes(unsafe.Pointer(p), n)
		return
	}
	randBytes(unsafe.Pointer(p+3), n-3)
	b0 := *(*byte)(unsafe.Pointer(p))
	*(*byte)(unsafe.Pointer(p)) = b0 &^ 0x01
}

//export HookGetIfTable2
func HookGetIfTable2(table *unsafe.Pointer, heap unsafe.Pointer) uint32 {
	r := callOrig("GetIfTable2", uintptr(unsafe.Pointer(table)), uintptr(heap))
	n := uint32(0)
	if r == 0 && table != nil && *table != nil {
		base := uintptr(*table)
		n = *(*uint32)(unsafe.Pointer(base)) // NumEntries
		if spoofEnabled && os.Getenv("SGTRACE_SPOOF_IFTABLE2") == "1" {
			const rowSize = 3852
			for i := uint32(0); i < n && i < 128; i++ {
				row := base + 8 + uintptr(i)*rowSize
				pl := *(*uint32)(unsafe.Pointer(row + 1056))
				if pl > 0 && pl <= 32 {
					spoofMAC(row+1060, int(pl))
				}
				ppl := *(*uint32)(unsafe.Pointer(row + 1092))
				if ppl > 0 && ppl <= 32 {
					spoofMAC(row+1096, int(ppl))
				}
			}
			logf("[SPOOF] IfTable2: %d MACs randomizados", n)
		}
	}
	logf("[GetIfTable2] -> r=%x entries=%d", r, n)
	return uint32(r)
}

//export HookGetNetworkParams
func HookGetNetworkParams(info unsafe.Pointer, size *uint32) uint32 {
	r := callOrig("GetNetworkParams", uintptr(info), uintptr(unsafe.Pointer(size)))
	logf("[GetNetworkParams] -> r=%x", r)
	return uint32(r)
}

//export HookGetSystemMetrics
func HookGetSystemMetrics(idx int32) int32 {
	r := callOrig("GetSystemMetrics", uintptr(idx))
	logf("[GetSystemMetrics] idx=%d -> %d", idx, int32(r))
	return int32(r)
}

//export HookSystemParametersInfoW
func HookSystemParametersInfoW(action uint32, param uint32, pv unsafe.Pointer, winini uint32) int32 {
	r := callOrig("SystemParametersInfoW", uintptr(action), uintptr(param), uintptr(pv), uintptr(winini))
	logf("[SystemParametersInfoW] action=%d", action)
	return int32(r)
}

//export HookCoCreateInstance
func HookCoCreateInstance(clsid unsafe.Pointer, unkOuter unsafe.Pointer, ctx uint32, riid unsafe.Pointer, out *unsafe.Pointer) uintptr {
	var cid [16]byte
	if clsid != nil {
		cid = *(*[16]byte)(clsid)
	}
	r := callOrig("CoCreateInstance", uintptr(clsid), uintptr(unkOuter), uintptr(ctx), uintptr(riid), uintptr(unsafe.Pointer(out)))
	logf("[CoCreateInstance] clsid=%x ctx=%x -> r=%x", cid, ctx, r)
	return r
}

//export HookCreateFileW
func HookCreateFileW(path *uint16, access uint32, share uint32, sa unsafe.Pointer, disp uint32, flags uint32, tmpl uintptr) uintptr {
	p := gowstr(path)
	r := callOrig("CreateFileW", uintptr(unsafe.Pointer(path)), uintptr(access), uintptr(share), uintptr(sa), uintptr(disp), uintptr(flags), tmpl)
	if verboseFiles {
		logf("[CreateFileW] %s -> h=%x", p, r)
	} else if hasPrefix(p, `\\.\`) || contains(lower(p), "physicaldrive") || contains(lower(p), "safestore") || contains(lower(p), "alibaba") {
		logf("[CreateFileW*] %s -> h=%x", p, r)
	}
	return r
}

//export HookDeviceIoControl
func HookDeviceIoControl(h uintptr, code uint32, in unsafe.Pointer, inSize uint32, out unsafe.Pointer, outSize uint32, ret *uint32, ov unsafe.Pointer) int32 {
	dev := code >> 16
	if dev == 0x2D || dev == 0x07 || dev == 0x09 || dev == 0x04 || dev == 0x56 || dev == 0x0B || dev == 0x0D {
		logf("[DeviceIoControl] h=%x code=%08x in=%d out=%d", h, code, inSize, outSize)
	}
	r := int32(callOrig("DeviceIoControl", h, uintptr(code), uintptr(in), uintptr(inSize), uintptr(out), uintptr(outSize), uintptr(unsafe.Pointer(ret)), uintptr(ov)))
	if spoofEnabled != false && r != 0 && out != nil && outSize > 8 {
		switch code {
		case 0x002d1400, 0x002d1080: // STORAGE_QUERY serial/property
			randBytes(unsafe.Pointer(uintptr(out)+4), int(outSize)-4)
			logf("[SPOOF] DIO %08x: %d bytes randomizados", code, outSize-4)
		case 0x00070000, 0x00070048: // geometry
			if outSize > 16 {
				randBytes(unsafe.Pointer(uintptr(out)+8), int(outSize)-8)
				logf("[SPOOF] DIO %08x: geometry randomizada", code)
			}
		}
	}
	return r
}

//export HookGetSystemFirmwareTables
func HookGetSystemFirmwareTables(sig uint32, buf unsafe.Pointer, size uint32) uint32 {
	logf("[GetSystemFirmwareTables] sig=%08x", sig)
	return uint32(callOrig("GetSystemFirmwareTables", uintptr(sig), uintptr(buf), uintptr(size)))
}

//export HookEnumSystemFirmwareTables
func HookEnumSystemFirmwareTables(sig uint32, buf unsafe.Pointer, size uint32) uint32 {
	logf("[EnumSystemFirmwareTables] sig=%08x", sig)
	return uint32(callOrig("EnumSystemFirmwareTables", uintptr(sig), uintptr(buf), uintptr(size)))
}

//export HookLoadLibraryExW
func HookLoadLibraryExW(name *uint16, file unsafe.Pointer, flags uint32) uintptr {
	logf("[LoadLibraryExW] %s flags=%x", gowstr(name), flags)
	r := callOrig("LoadLibraryExW", uintptr(unsafe.Pointer(name)), uintptr(file), uintptr(flags))
	if r != 0 {
		patchModule(r)
	}
	return r
}

//export HookLoadLibraryW
func HookLoadLibraryW(name *uint16) uintptr {
	logf("[LoadLibraryW] %s", gowstr(name))
	r := callOrig("LoadLibraryW", uintptr(unsafe.Pointer(name)))
	if r != 0 {
		patchModule(r)
	}
	return r
}

//export HookGetProcAddress
func HookGetProcAddress(mod uintptr, name *byte) uintptr {
	r := callOrig("GetProcAddress", mod, uintptr(unsafe.Pointer(name)))
	n := gostr(name)
	if isHookedName(n) {
		logf("[GetProcAddress] %s -> redirecionado", n)
		if h, ok := hookAddr[n]; ok {
			return h
		}
	}
	return r
}

//export HookGetAddrInfo
func HookGetAddrInfo(node *uint16, service *uint16, hints unsafe.Pointer, res *unsafe.Pointer) int32 {
	logf("[getaddrinfo] node=%s service=%s", gowstr(node), gowstr(service))
	return int32(callOrig("getaddrinfo", uintptr(unsafe.Pointer(node)), uintptr(unsafe.Pointer(service)), uintptr(hints), uintptr(unsafe.Pointer(res))))
}

//export HookGetHostByName
func HookGetHostByName(name *byte) unsafe.Pointer {
	logf("[gethostbyname] %s", gostr(name))
	r := callOrig("gethostbyname", uintptr(unsafe.Pointer(name)))
	return unsafe.Pointer(r)
}

//export HookConnect
func HookConnect(s uintptr, addr unsafe.Pointer, namelen int32) int32 {
	fam := uint16(0)
	port := uint16(0)
	ip4 := uint32(0)
	if addr != nil {
		fam = *(*uint16)(addr)
		b := (*[16]byte)(addr)
		port = uint16(b[2])<<8 | uint16(b[3])
		if fam == 2 && len(b) >= 8 {
			ip4 = uint32(b[4]) | uint32(b[5])<<8 | uint32(b[6])<<16 | uint32(b[7])<<24
		}
	}
	logf("[connect] fam=%d %d.%d.%d.%d:%d", fam, ip4&0xff, (ip4>>8)&0xff, (ip4>>16)&0xff, (ip4>>24)&0xff, port)
	return int32(callOrig("connect", s, uintptr(addr), uintptr(namelen)))
}

//export HookNtQuerySystemInformation
func HookNtQuerySystemInformation(class uint32, info unsafe.Pointer, length uint32, ret *uint32) uintptr {
	logf("[NtQuerySystemInformation] class=%d len=%d", class, length)
	r := callOrig("NtQuerySystemInformation", uintptr(class), uintptr(info), uintptr(length), uintptr(unsafe.Pointer(ret)))
	if spoofEnabled && r == 0 && class == 1 && info != nil && length >= 16 {
		// SystemProcessorInformation: patch FeatureBits @8
		randBytes(unsafe.Pointer(uintptr(info)+8), 8)
		logf("[SPOOF] ProcessorInformation features")
	}
	return r
}

func oaName(oa unsafe.Pointer) string {
	if oa == nil {
		return "<no-oa>"
	}
	us := *(*uintptr)(unsafe.Pointer(uintptr(oa) + 16))
	if us == 0 {
		return "<no-name>"
	}
	blen := *(*uint16)(unsafe.Pointer(us))
	buf := *(*uintptr)(unsafe.Pointer(us + 8))
	if buf == 0 || blen == 0 {
		return "<empty>"
	}
	n := make([]uint16, blen/2)
	for i := range n {
		n[i] = *(*uint16)(unsafe.Pointer(buf + uintptr(i)*2))
	}
	return syscall.UTF16ToString(n)
}

//export HookNtCreateFile
func HookNtCreateFile(fh **uintptr, access uint32, oa unsafe.Pointer, iosb unsafe.Pointer, alloc unsafe.Pointer, attrib uint32, share uint32, disp uint32, opts uint32, ea unsafe.Pointer, eaLen uint32) uintptr {
	logf("[NtCreateFile] %s", oaName(oa))
	return callOrig("NtCreateFile", uintptr(unsafe.Pointer(fh)), uintptr(access), uintptr(oa), uintptr(iosb),
		uintptr(alloc), uintptr(attrib), uintptr(share), uintptr(disp), uintptr(opts), uintptr(ea), uintptr(eaLen))
}

//export HookNtOpenKey
func HookNtOpenKey(kh **uintptr, access uint32, oa unsafe.Pointer) uintptr {
	logf("[NtOpenKey] %s", oaName(oa))
	return callOrig("NtOpenKey", uintptr(unsafe.Pointer(kh)), uintptr(access), uintptr(oa))
}

//export HookNtOpenKeyEx
func HookNtOpenKeyEx(kh **uintptr, access uint32, oa unsafe.Pointer, opts uint32) uintptr {
	logf("[NtOpenKeyEx] %s", oaName(oa))
	return callOrig("NtOpenKeyEx", uintptr(unsafe.Pointer(kh)), uintptr(access), uintptr(oa), uintptr(opts))
}

func usNameToString(us unsafe.Pointer) string {
	if us == nil {
		return "<nil>"
	}
	blen := *(*uint16)(us)
	buf := *(*uintptr)(unsafe.Pointer(uintptr(us) + 8))
	if buf == 0 || blen == 0 {
		return "<empty>"
	}
	n := make([]uint16, blen/2)
	for i := range n {
		n[i] = *(*uint16)(unsafe.Pointer(buf + uintptr(i)*2))
	}
	return syscall.UTF16ToString(n)
}

//export HookNtQueryValueKey
func HookNtQueryValueKey(kh uintptr, name unsafe.Pointer, class uint32, info unsafe.Pointer, length uint32, ret *uint32) uintptr {
	logf("[NtQueryValueKey] h=%x name=%s class=%d", kh, usNameToString(name), class)
	return callOrig("NtQueryValueKey", kh, uintptr(name), uintptr(class), uintptr(info), uintptr(length), uintptr(unsafe.Pointer(ret)))
}

//export HookRtlGetVersion
func HookRtlGetVersion(v unsafe.Pointer) uintptr {
	logf("[RtlGetVersion]")
	return callOrig("RtlGetVersion", uintptr(v))
}
