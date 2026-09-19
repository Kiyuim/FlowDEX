git clone git@github.com:dreamerinsgp/solana-anchor-go-new.git

cd solana-anchor-go-new

go build

./solana-anchor-go -src=/home/ubuntu/dex_full/fun_dex_v2/pkg/pumpfun/pump/idl.json -pkg=pump -dst=/home/ubuntu/dex_full/fun_dex_v2/pkg/pumpfun/pump/idl/generated/pump
