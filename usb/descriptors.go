package usb

const (
	// Vendor specific: https://www.usb.org/defined-class-codes#anchor_BaseClassFFh
	bdbClass    = 0xff
	bdbSubclass = 42
	bdbProtocol = 1
)

const (
	maxPacketSizeFs = 64
	maxPacketSizeHs = 512
	maxPacketSizeSs = 1024
)

const bdbStrInterface = "balena Device Bridge (BDB)"

var fsDescriptors = &descriptors{
	intf: interfaceDescriptor{
		interfaceNumber: 0,
		endpointsNumber: 2,
		class:           bdbClass,
		subClass:        bdbSubclass,
		protocol:        bdbProtocol,
		intf:            1, // First string from the provided table.
	},
	source: endpointDescriptor{
		endpointAddress: 1 | DirectionOut,
		attributes:      usbEndpointXferBulk,
		maxPacketSize:   maxPacketSizeFs,
	},
	sink: endpointDescriptor{
		endpointAddress: 2 | DirectionIn,
		attributes:      usbEndpointXferBulk,
		maxPacketSize:   maxPacketSizeFs,
	},
}

var hsDescriptor = &descriptors{
	intf: fsDescriptors.intf,
	source: endpointDescriptor{
		endpointAddress: 1 | DirectionOut,
		attributes:      usbEndpointXferBulk,
		maxPacketSize:   maxPacketSizeHs,
	},
	sink: endpointDescriptor{
		endpointAddress: 2 | DirectionIn,
		attributes:      usbEndpointXferBulk,
		maxPacketSize:   maxPacketSizeHs,
	},
}

// TODO: Implement?
var ssDescriptor *descriptors
