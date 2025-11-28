package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
)

// AsterixMessage represents a decoded ASTERIX message
type AsterixMessage struct {
	Category    int                      `json:"category"`
	Length      int                      `json:"length"`
	DataBlocks  []map[string]interface{} `json:"data_blocks,omitempty"`
	RawFSPEC    string                   `json:"raw_fspec,omitempty"`
	ParseError  string                   `json:"parse_error,omitempty"`
	Unsupported bool                     `json:"unsupported,omitempty"`
}

// isAsterixMessage checks if the payload appears to be an ASTERIX message
func isAsterixMessage(payload []byte) bool {
	if len(payload) < 3 {
		return false
	}

	// Check category (typically between 1-255, common ones are 1-250)
	category := int(payload[0])
	if category == 0 || category > 250 {
		return false
	}

	// Check length field (bytes 1-2, big-endian)
	length := int(binary.BigEndian.Uint16(payload[1:3]))

	// Length should match actual payload length or be close to it
	// (within reasonable bounds for fragmented messages)
	if length < 3 || length > len(payload)+100 {
		return false
	}

	// If length matches payload exactly or payload is at least 3 bytes, likely ASTERIX
	if length == len(payload) || (len(payload) >= 3 && length <= len(payload)*2) {
		return true
	}

	return false
}

// decodeAsterixMessage attempts to decode an ASTERIX message
func decodeAsterixMessage(payload []byte) *AsterixMessage {
	if len(payload) < 3 {
		return &AsterixMessage{
			ParseError: "payload too short for ASTERIX",
		}
	}

	msg := &AsterixMessage{
		Category:   int(payload[0]),
		Length:     int(binary.BigEndian.Uint16(payload[1:3])),
		DataBlocks: make([]map[string]interface{}, 0),
	}

	// Validate length
	if msg.Length < 3 || msg.Length > len(payload) {
		msg.ParseError = fmt.Sprintf("invalid length field: %d (payload size: %d)", msg.Length, len(payload))
		return msg
	}

	// Parse data blocks starting at offset 3
	offset := 3
	blockNum := 0

	for offset < msg.Length && offset < len(payload) {
		block, bytesRead, err := decodeDataBlock(payload[offset:], msg.Category)
		if err != nil {
			msg.ParseError = fmt.Sprintf("error at block %d, offset %d: %v", blockNum, offset, err)
			break
		}

		if bytesRead == 0 {
			break
		}

		msg.DataBlocks = append(msg.DataBlocks, block)
		offset += bytesRead
		blockNum++
	}

	return msg
}

// decodeDataBlock decodes a single ASTERIX data block
func decodeDataBlock(data []byte, category int) (map[string]interface{}, int, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("empty data block")
	}

	block := make(map[string]interface{})
	offset := 0

	// Parse FSPEC (Field Specification)
	fspec, fspecLen := parseFSPEC(data)
	if fspecLen == 0 {
		return nil, 0, fmt.Errorf("failed to parse FSPEC")
	}

	block["fspec"] = base64.StdEncoding.EncodeToString(data[:fspecLen])
	offset += fspecLen

	// Decode data items based on FSPEC and category
	dataItems := make(map[string]interface{})
	frn := 1 // Field Reference Number

	// Process ALL FSPEC bytes (each byte has 7 data item bits + 1 FX bit)
	for byteIdx := 0; byteIdx < len(fspec); byteIdx++ {
		fspecByte := fspec[byteIdx]
		for bitIdx := 7; bitIdx >= 1; bitIdx-- { // bits 7-1 (bit 0 is FX - extension bit)
			if fspecByte&(1<<bitIdx) != 0 {
				// This FRN is present
				fieldName, fieldValue, bytesRead := decodeDataItem(data[offset:], category, frn)
				if bytesRead > 0 {
					dataItems[fieldName] = fieldValue
					offset += bytesRead
				}
			}
			frn++
		}
	}

	if len(dataItems) > 0 {
		block["data_items"] = dataItems
	}

	return block, offset, nil
}

// parseFSPEC parses the Field Specification (variable length bitmap)
func parseFSPEC(data []byte) ([]byte, int) {
	fspec := make([]byte, 0)

	for i := 0; i < len(data); i++ {
		fspec = append(fspec, data[i])
		// Check FX bit (bit 0) - if 0, this is the last FSPEC byte
		if data[i]&0x01 == 0 {
			return fspec, i + 1
		}
		// Prevent infinite loop on malformed data
		if i >= 10 {
			return fspec, i + 1
		}
	}

	return fspec, len(fspec)
}

// decodeDataItem decodes a specific data item based on category and FRN
func decodeDataItem(data []byte, category int, frn int) (string, interface{}, int) {
	fieldName := fmt.Sprintf("I%03d_%03d", category, frn)

	// Category-specific decoding
	switch category {
	case 48: // Monoradar Target Reports
		return decodeCAT48Item(data, frn)
	case 62: // System Track Data
		return decodeCAT62Item(data, frn)
	case 34: // Monosensor Surface Movement Data
		return decodeCAT34Item(data, frn)
	case 21: // ADS-B Target Reports
		return decodeCAT21Item(data, frn)
	default:
		// Unknown category - try to read a reasonable amount
		size := estimateFieldSize(data)
		if size > 0 && size <= len(data) {
			return fieldName, base64.StdEncoding.EncodeToString(data[:size]), size
		}
		return fieldName, base64.StdEncoding.EncodeToString(data[:min(len(data), 8)]), min(len(data), 8)
	}
}

// decodeCAT48Item decodes CAT 048 data items
func decodeCAT48Item(data []byte, frn int) (string, interface{}, int) {
	switch frn {
	case 1: // I048/010 - Data Source Identifier
		if len(data) >= 2 {
			return "data_source_id", map[string]interface{}{
				"sac": int(data[0]),
				"sic": int(data[1]),
			}, 2
		}
	case 3: // I048/040 - Measured Position in Polar Co-ordinates
		if len(data) >= 4 {
			rho := float64(binary.BigEndian.Uint16(data[0:2])) * (1.0 / 256.0) // NM
			theta := float64(binary.BigEndian.Uint16(data[2:4])) * (360.0 / 65536.0) // degrees
			return "measured_position_polar", map[string]interface{}{
				"rho_nm":      rho,
				"theta_deg":   theta,
			}, 4
		}
	case 4: // I048/070 - Mode-3/A Code
		if len(data) >= 2 {
			v := binary.BigEndian.Uint16(data[0:2])
			mode3a := ((v & 0x0FFF) >> 0)
			return "mode3a", map[string]interface{}{
				"validated": (v & 0x8000) == 0,
				"garbled":   (v & 0x4000) != 0,
				"code":      fmt.Sprintf("%04o", mode3a),
			}, 2
		}
	case 5: // I048/090 - Flight Level
		if len(data) >= 2 {
			flRaw := binary.BigEndian.Uint16(data[0:2])
			flValue := int16(flRaw & 0x3FFF)
			if flRaw&0x2000 != 0 { // Check sign bit (bit 13)
				flValue = -((^flValue + 1) & 0x3FFF)
			}
			return "flight_level", map[string]interface{}{
				"validated": (flRaw & 0x8000) == 0,
				"garbled":   (flRaw & 0x4000) != 0,
				"fl":        float64(flValue) / 4.0,
			}, 2
		}
	case 8: // I048/220 - Aircraft Address
		if len(data) >= 3 {
			addr := (uint32(data[0]) << 16) | (uint32(data[1]) << 8) | uint32(data[2])
			return "aircraft_address", fmt.Sprintf("%06X", addr), 3
		}
	case 9: // I048/240 - Aircraft Identification
		if len(data) >= 6 {
			callsign := decodeAircraftID(data[:6])
			return "aircraft_id", callsign, 6
		}
	}

	// Default: encode as base64
	size := estimateFieldSize(data)
	return fmt.Sprintf("I048_%03d", frn), base64.StdEncoding.EncodeToString(data[:size]), size
}

// decodeCAT62Item decodes CAT 062 data items
func decodeCAT62Item(data []byte, frn int) (string, interface{}, int) {
	switch frn {
	case 1: // I062/010 - Data Source Identifier
		if len(data) >= 2 {
			return "data_source_id", map[string]interface{}{
				"sac": int(data[0]),
				"sic": int(data[1]),
			}, 2
		}
	case 4: // I062/040 - Track Number
		if len(data) >= 2 {
			trackNum := binary.BigEndian.Uint16(data[0:2])
			return "track_number", int(trackNum), 2
		}
	case 8: // I062/105 - Calculated Position (WGS-84)
		if len(data) >= 8 {
			lat := int32(binary.BigEndian.Uint32(data[0:4]))
			lon := int32(binary.BigEndian.Uint32(data[4:8]))
			return "position_wgs84", map[string]interface{}{
				"latitude":  float64(lat) * (180.0 / math.Pow(2, 31)),
				"longitude": float64(lon) * (180.0 / math.Pow(2, 31)),
			}, 8
		}
	case 10: // I062/136 - Measured Flight Level
		if len(data) >= 2 {
			fl := int16(binary.BigEndian.Uint16(data[0:2]))
			return "measured_flight_level", float64(fl) * 0.25, 2
		}
	}

	// Default: encode as base64
	size := estimateFieldSize(data)
	return fmt.Sprintf("I062_%03d", frn), base64.StdEncoding.EncodeToString(data[:size]), size
}

// decodeCAT34Item decodes CAT 034 data items
func decodeCAT34Item(data []byte, frn int) (string, interface{}, int) {
	switch frn {
	case 1: // I034/010 - Data Source Identifier
		if len(data) >= 2 {
			return "data_source_id", map[string]interface{}{
				"sac": int(data[0]),
				"sic": int(data[1]),
			}, 2
		}
	}

	size := estimateFieldSize(data)
	return fmt.Sprintf("I034_%03d", frn), base64.StdEncoding.EncodeToString(data[:size]), size
}

// decodeCAT21Item decodes CAT 021 data items (ADS-B Target Reports)
func decodeCAT21Item(data []byte, frn int) (string, interface{}, int) {
	switch frn {
	case 1: // I021/010 - Data Source Identification
		if len(data) >= 2 {
			return "data_source_id", map[string]interface{}{
				"sac": int(data[0]),
				"sic": int(data[1]),
			}, 2
		}
	case 2: // I021/040 - Target Report Descriptor (variable length)
		size := 1
		for i := 0; i < len(data) && i < 10; i++ {
			if data[i]&0x01 == 0 {
				break
			}
			size++
		}
		if size <= len(data) {
			return "target_report_descriptor", base64.StdEncoding.EncodeToString(data[:size]), size
		}
	case 3: // I021/161 - Track Number
		if len(data) >= 2 {
			trackNum := binary.BigEndian.Uint16(data[0:2]) & 0x0FFF // 12 bits
			return "track_number", int(trackNum), 2
		}
	case 4: // I021/015 - Service Identification
		if len(data) >= 1 {
			return "service_id", int(data[0]), 1
		}
	case 5: // I021/071 - Time of Applicability for Position
		if len(data) >= 3 {
			toa := (uint32(data[0]) << 16) | (uint32(data[1]) << 8) | uint32(data[2])
			return "time_of_applicability_position", map[string]interface{}{
				"raw":     toa,
				"seconds": float64(toa) / 128.0,
			}, 3
		}
	case 6: // I021/130 - Position in WGS-84 Coordinates
		if len(data) >= 8 {
			lat := int32(binary.BigEndian.Uint32(data[0:4]))
			lon := int32(binary.BigEndian.Uint32(data[4:8]))
			return "position_wgs84", map[string]interface{}{
				"latitude":  float64(lat) * (180.0 / math.Pow(2, 23)),
				"longitude": float64(lon) * (180.0 / math.Pow(2, 23)),
			}, 8
		}
	case 7: // I021/131 - High-Resolution Position in WGS-84
		if len(data) >= 8 {
			lat := int32(binary.BigEndian.Uint32(data[0:4]))
			lon := int32(binary.BigEndian.Uint32(data[4:8]))
			return "position_wgs84_high_res", map[string]interface{}{
				"latitude":  float64(lat) * (180.0 / math.Pow(2, 30)),
				"longitude": float64(lon) * (180.0 / math.Pow(2, 30)),
			}, 8
		}
	case 11: // I021/080 - Target Address (24-bit ICAO address) - in 2nd FSPEC byte
		if len(data) >= 3 {
			addr := (uint32(data[0]) << 16) | (uint32(data[1]) << 8) | uint32(data[2])
			return "target_address", fmt.Sprintf("%06X", addr), 3
		}
	case 16: // I021/146 - Selected Altitude (appears later in FSPEC)
		if len(data) >= 2 {
			alt := int16(binary.BigEndian.Uint16(data[0:2]))
			return "selected_altitude", map[string]interface{}{
				"source":   (alt >> 15) & 0x01,
				"altitude": float64(alt&0x7FFF) * 25.0, // feet
			}, 2
		}
	case 17: // I021/148 - Final State Selected Altitude
		if len(data) >= 2 {
			alt := int16(binary.BigEndian.Uint16(data[0:2]))
			return "final_state_selected_altitude", map[string]interface{}{
				"mv":       (alt >> 15) & 0x01,
				"ah":       (alt >> 14) & 0x01,
				"am":       (alt >> 13) & 0x01,
				"altitude": float64(alt&0x1FFF) * 25.0, // feet
			}, 2
		}
	case 20: // I021/110 - Trajectory Intent (appears later, variable)
		// Variable length compound field - complex structure
		if len(data) >= 1 {
			// This is complex - just encode as base64 for now
			estimatedSize := estimateFieldSize(data)
			return "trajectory_intent", base64.StdEncoding.EncodeToString(data[:estimatedSize]), estimatedSize
		}
	case 22: // I021/170 - Target Identification (aircraft ID/callsign)
		if len(data) >= 6 {
			callsign := decodeAircraftID(data[:6])
			return "target_identification", callsign, 6
		}
	case 23: // I021/020 - Emitter Category
		if len(data) >= 1 {
			return "emitter_category", int(data[0]), 1
		}
	}

	size := estimateFieldSize(data)
	return fmt.Sprintf("I021_%03d", frn), base64.StdEncoding.EncodeToString(data[:size]), size
}

// decodeAircraftID decodes 6-byte aircraft identification (callsign)
func decodeAircraftID(data []byte) string {
	if len(data) < 6 {
		return ""
	}

	callsign := make([]byte, 8)
	chars := "?ABCDEFGHIJKLMNOPQRSTUVWXYZ????? ???????????????0123456789??????"

	// Unpack 6-bit characters
	callsign[0] = chars[(data[0]>>2)&0x3F]
	callsign[1] = chars[((data[0]&0x03)<<4)|((data[1]>>4)&0x0F)]
	callsign[2] = chars[((data[1]&0x0F)<<2)|((data[2]>>6)&0x03)]
	callsign[3] = chars[data[2]&0x3F]
	callsign[4] = chars[(data[3]>>2)&0x3F]
	callsign[5] = chars[((data[3]&0x03)<<4)|((data[4]>>4)&0x0F)]
	callsign[6] = chars[((data[4]&0x0F)<<2)|((data[5]>>6)&0x03)]
	callsign[7] = chars[data[5]&0x3F]

	// Trim trailing spaces
	result := string(callsign)
	for len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}

	return result
}

// estimateFieldSize tries to estimate the size of an unknown field
func estimateFieldSize(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	// Check if it's a variable-length field (bit 0 is FX - extension indicator)
	if data[0]&0x01 != 0 && len(data) > 1 {
		// Variable length - read until FX bit is 0
		for i := 0; i < len(data) && i < 20; i++ {
			if data[i]&0x01 == 0 {
				return i + 1
			}
		}
	}

	// Default to common fixed sizes
	if len(data) >= 2 {
		return 2
	}
	return 1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
