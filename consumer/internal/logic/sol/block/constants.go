package block

import (
	"errors"

	"dex/pkg/constants"

	"github.com/blocto/solana-go-sdk/common"
)

const SolChainId = constants.SolChainId
const SolChainIdInt = constants.SolChainIdInt

const ProgramStrRaydiumV4 = constants.ProgramStrRaydiumV4
const ProgramStrRaydiumV2 = constants.ProgramStrRaydiumV2
const ProgramStrToken = constants.ProgramStrToken

const ProgramStrPumpFun = constants.ProgramStrPumpFun
const ProgramStrPumpAmm = constants.ProgramStrPumpAmm
const ProgramStrPumpMeteora = constants.ProgramStrPumpMeteora
const ProgramStrPumpMeteoraOpt = constants.ProgramStrPumpMeteoraOpt

const TokenStrWrapSol = constants.TokenStrWrapSol
const TokenStrUSDC = constants.TokenStrUSDC
const TokenStrUSDT = constants.TokenStrUSDT

var ProgramRaydiumV4 = common.PublicKeyFromString(ProgramStrRaydiumV4)
var ProgramRaydiumV2 = common.PublicKeyFromString(ProgramStrRaydiumV2)

var ProgramMap = make(map[string]common.PublicKey)
var ErrNotSupportWarp = errors.New("not support swap")
var ErrNotSupportInstruction = errors.New("not support instruction")
var ErrTokenAmountIsZero = errors.New("tokenAmount is zero,")
var ErrServiceStop = errors.New("service stop")
var ErrUnknownProgram = errors.New("unknown program")
