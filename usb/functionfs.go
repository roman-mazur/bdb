package usb

import (
	"encoding/binary"
	"io"
)

const (
	functionfsStringsMagic       = 2
	functionfsDescriptorsMagicV2 = 3
)

const (
	functionfsHasFsDesc = 1
	functionfsHasHsDesc = 2
	functionfsHasSsDesc = 4
)

type descriptorType = byte

const (
	dtInterface descriptorType = 0x04
	dtEndpoint  descriptorType = 0x05
)

type Direction = uint8

const (
	DirectionOut Direction = 0    // To device.
	DirectionIn  Direction = 0x80 // To host.
)

const (
	usbEndpointXferBulk = 2
)

var le = binary.LittleEndian

type descriptors struct {
	intf   interfaceDescriptor
	sink   endpointDescriptor
	source endpointDescriptor
}

func (d *descriptors) Len() int {
	return descLength(&d.intf) + descLength(&d.sink) + descLength(&d.source)
}

func (d *descriptors) Marshal(dst []byte) int {
	offset := setDescriptor(dst, &d.intf)
	offset += setDescriptor(dst[offset:], &d.sink)
	offset += setDescriptor(dst[offset:], &d.source)
	return offset
}

type baseDescriptor interface {
	payload() []byte
	dtype() descriptorType
}

// From uapi/linux/usb/ch9.h
// See https://github.com/torvalds/linux/blob/6f0d349d922ba44e4348a17a78ea51b7135965b1/include/uapi/linux/usb/ch9.h

//struct usb_interface_descriptor {
//	__u8  bLength;
//	__u8  bDescriptorType;
//
//	__u8  bInterfaceNumber;
//	__u8  bAlternateSetting;
//	__u8  bNumEndpoints;
//	__u8  bInterfaceClass;
//	__u8  bInterfaceSubClass;
//	__u8  bInterfaceProtocol;
//	__u8  iInterface;
//} __attribute__ ((packed));

type interfaceDescriptor struct {
	interfaceNumber  uint8
	alternateSetting uint8
	endpointsNumber  uint8
	class            uint8
	subClass         uint8
	protocol         uint8
	intf             uint8
}

func (id *interfaceDescriptor) payload() []byte {
	return []byte{
		id.interfaceNumber,
		id.alternateSetting,
		id.endpointsNumber,
		id.class,
		id.subClass,
		id.protocol,
		id.intf,
	}
}

func (id *interfaceDescriptor) dtype() descriptorType {
	return dtInterface
}

type endpointDescriptor struct {
	endpointAddress uint8
	attributes      uint8
	maxPacketSize   uint16
	interval        uint8
}

func (ed *endpointDescriptor) payload() []byte {
	res := []byte{
		ed.endpointAddress,
		ed.attributes,
		0, 0, // Max packet size placeholder.
		ed.interval,
	}
	le.PutUint16(res[2:], ed.maxPacketSize)
	return res
}

func (ed *endpointDescriptor) dtype() descriptorType {
	return dtEndpoint
}

func descLength(desc baseDescriptor) int {
	return len(desc.payload()) + 2
}

func setDescriptor(dst []byte, desc baseDescriptor) int {
	payload := desc.payload()
	length := len(payload) + 2
	_ = dst[length-1] // Boundary checks.

	dst[0] = uint8(length)
	dst[1] = desc.dtype()
	copy(dst[2:], payload)
	return length
}

func setDescHeader(dst []byte, length uint32) {
	// From uapi/linux/usb/functionfs.h
	// See https://github.com/torvalds/linux/blob/6f0d349d922ba44e4348a17a78ea51b7135965b1/include/uapi/linux/usb/functionfs.h

	//struct usb_functionfs_descs_head_v2 {
	//	__le32 magic;
	//	__le32 length;
	//	__le32 flags;
	//	/*
	//	 * __le32 fs_count, hs_count, fs_count; must be included manually in
	//	 * the structure taking flags into consideration.
	//   */
	//} __attribute__((packed));

	_ = dst[11] // Enforce boundaries check.
	le.PutUint32(dst, functionfsDescriptorsMagicV2)
	le.PutUint32(dst[4:], length)
	flags := uint32(0)
	if fsDescriptors != nil {
		flags |= functionfsHasFsDesc
	}
	if hsDescriptor != nil {
		flags |= functionfsHasHsDesc
	}
	if ssDescriptor != nil {
		flags |= functionfsHasSsDesc
	}
	le.PutUint32(dst[8:], flags)
}

func BuildUsbDescriptors() []byte {
	// Simplest example of descriptors.

	//static const struct {
	//  struct usb_functionfs_descs_head_v2 header;
	//  __le32 fs_count;
	//  __le32 hs_count;
	//  struct {
	//    struct usb_interface_descriptor intf;
	//    struct usb_endpoint_descriptor_no_audio bulk_sink;
	//    struct usb_endpoint_descriptor_no_audio bulk_source;
	//  } __attribute__ ((__packed__)) fs_descs, hs_descs;
	//} __attribute__ ((__packed__));

	hLength := 12

	descCount := 0
	descLen := 0
	if fsDescriptors != nil {
		descCount++
		descLen += fsDescriptors.Len()
	}
	if hsDescriptor != nil {
		descCount++
		descLen += hsDescriptor.Len()
	}
	if ssDescriptor != nil {
		descCount++
		descLen += ssDescriptor.Len()
	}

	res := make([]byte, hLength+descCount*4+descLen)
	setDescHeader(res, uint32(len(res)))
	offset := hLength
	for i := 0; i < descCount; i++ {
		le.PutUint32(res[offset:], 3) // FS/HS/SS desc count.
		offset += 4
	}

	if fsDescriptors != nil {
		offset += fsDescriptors.Marshal(res[offset:])
	}
	if hsDescriptor != nil {
		offset += hsDescriptor.Marshal(res[offset:])
	}
	if ssDescriptor != nil {
		offset += ssDescriptor.Marshal(res[offset:])
	}

	if offset != len(res) {
		panic("inconsistent ffs descriptors")
	}
	return res
}

func setStringsHeader(dst []byte, length uint32) {
	// From uapi/linux/usb/functionfs.h
	// See https://github.com/torvalds/linux/blob/6f0d349d922ba44e4348a17a78ea51b7135965b1/include/uapi/linux/usb/functionfs.h

	//struct usb_functionfs_strings_head {
	//	__le32 magic;
	//	__le32 length;
	//	__le32 str_count;
	//	__le32 lang_count;
	//} __attribute__((packed));

	le.PutUint32(dst, functionfsStringsMagic)
	le.PutUint32(dst[4:], length)
	le.PutUint32(dst[8:], 1)
	le.PutUint32(dst[12:], 1)
}

func BuildUsbStrings() []byte {
	hLength := 16
	res := make([]byte, hLength+2+len(bdbStrInterface)+1)
	setStringsHeader(res, uint32(len(res)))
	le.PutUint16(res[hLength:], 0x0409)    // en-us.
	copy(res[hLength+2:], bdbStrInterface) // Go string literal is utf-8.
	return res
}

type FfsEventType int

const (
	FunctionfsBind FfsEventType = iota
	FunctionfsUnbind

	FunctionfsEnable
	FunctionfsDisable

	FunctionfsSetup

	FunctionfsSuspend
	FunctionfsResume
)

func (t FfsEventType) String() string {
	switch t {
	case FunctionfsBind:
		return "BIND"
	case FunctionfsUnbind:
		return "UNBIND"
	case FunctionfsEnable:
		return "ENABLE"
	case FunctionfsDisable:
		return "DISABLE"
	case FunctionfsSetup:
		return "SETUP"
	case FunctionfsSuspend:
		return "SUSPEND"
	case FunctionfsResume:
		return "RESUME"
	default:
		return "BAD"
	}
}

type ControlRequestType byte

type FfsEvent struct {
	Type FfsEventType

	SetupPayload struct {
		RequestType uint8
		Request     uint8
		Value       uint16
		Index       uint16
		Length      uint16
	}
}

func ReadFfsEvent(input io.Reader) (*FfsEvent, error) {
	//struct usb_ctrlrequest {
	//	__u8 bRequestType;
	//	__u8 bRequest;
	//	__le16 wValue;
	//	__le16 wIndex;
	//	__le16 wLength;
	//} __attribute__ ((packed));

	//struct usb_functionfs_event {
	//	union {
	//		/* SETUP: packet; DATA phase i/o precedes next event
	//		 *(setup.bmRequestType & USB_DIR_IN) flags direction */
	//		struct usb_ctrlrequest	setup;
	//	} __attribute__((packed)) u;
	//
	//	/* enum usb_functionfs_event_type */
	//	__u8				type;
	//	__u8				_pad[3];
	//} __attribute__((packed));

	toRead := 12
	buf := make([]byte, toRead)
	for toRead > 0 {
		n, err := input.Read(buf[len(buf)-toRead:])
		if err != nil {
			return nil, err
		}
		toRead -= n
	}
	res := &FfsEvent{Type: FfsEventType(buf[8])}
	res.SetupPayload.RequestType = buf[0]
	res.SetupPayload.Request = buf[1]
	res.SetupPayload.Value = le.Uint16(buf[2:])
	res.SetupPayload.Index = le.Uint16(buf[4:])
	res.SetupPayload.Length = le.Uint16(buf[6:])
	return res, nil
}
