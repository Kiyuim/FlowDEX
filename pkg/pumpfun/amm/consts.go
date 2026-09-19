package amm

import "github.com/gagliardetto/solana-go"

var (
	// Global config account address for pumpfun swap
	PumpAmmGlobalConfigAddress = solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw")
	// Event authority address for pumpfun swap
	PumpAmmEventAuthorityAddress = solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR")
	// Protocol fee recipient 0x00
	ProtocolFeeRecipient0x00 = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	// Protocol fee recipient 0x01
	ProtocolFeeRecipient0x01 = solana.MustPublicKeyFromBase58("7VtfL8fvgNfhz17qKRMjzQEXgbdpnHHHQRh54R9jP2RJ")
	// Protocol fee recipient 0x02
	ProtocolFeeRecipient0x02 = solana.MustPublicKeyFromBase58("7hTckgnGnLQR6sdH7YkqFTAA7VwTfYFaZ6EhEsU3saCX")
	// Protocol fee recipient 0x03
	ProtocolFeeRecipient0x03 = solana.MustPublicKeyFromBase58("9rPYyANsfQZw3DnDmKE3YCQF5E8oD89UXoHn9JFEhJUz")
	// Protocol fee recipient 0x04
	ProtocolFeeRecipient0x04 = solana.MustPublicKeyFromBase58("AVmoTthdrX6tKt4nDjco2D775W2YK3sDhxPcMmzUAmTY")
	// Protocol fee recipient 0x05
	ProtocolFeeRecipient0x05 = solana.MustPublicKeyFromBase58("FWsW1xNtWscwNmKv6wVsU1iTzRN6wmmk3MjxRP5tT7hz")
	// Protocol fee recipient 0x06
	ProtocolFeeRecipient0x06 = solana.MustPublicKeyFromBase58("G5UZAVbAf46s7cKWoyKu8kYTip9DGTpbLZ2qa9Aq69dP")
	// Protocol fee recipient 0x07
	ProtocolFeeRecipient0x07 = solana.MustPublicKeyFromBase58("G5UZAVbAf46s7cKWoyKu8kYTip9DGTpbLZ2qa9Aq69dP")
)

var (
	ProtocolFeeRecipients = [8]solana.PublicKey{
		ProtocolFeeRecipient0x00,
		ProtocolFeeRecipient0x01,
		ProtocolFeeRecipient0x02,
		ProtocolFeeRecipient0x03,
		ProtocolFeeRecipient0x04,
		ProtocolFeeRecipient0x05,
		ProtocolFeeRecipient0x06,
		ProtocolFeeRecipient0x07,
	}
)
