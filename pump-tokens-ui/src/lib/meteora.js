// Client-side create_bonding_curve for our own pump-meteora programs (original + optimized).
// The program's `create_bonding_curve` instruction itself inits the mint, mints the whole
// supply to the global vault ATA, writes metadata and revokes mint authority — so the client
// only builds this ONE instruction (signed by the creator wallet + a fresh mint keypair).
//
// Account layout + discriminator are the same for both programs (verified end-to-end by the
// devnet seed scripts). Token config (decimals=6, supply) comes from each program's on-chain
// Config, NOT from the form.
import {
  PublicKey, SystemProgram, TransactionInstruction, SYSVAR_RENT_PUBKEY,
} from '@solana/web3.js';
import {
  getAssociatedTokenAddressSync, TOKEN_PROGRAM_ID, ASSOCIATED_TOKEN_PROGRAM_ID,
} from '@solana/spl-token';

// program ids keyed by launch target
export const METEORA_PROGRAMS = {
  meteora: new PublicKey('AEBUS7kBka3pg5HyzUqgDYspvAPjFryyXjA5ZvRhUJU5'),
  meteorav2: new PublicKey('241xjmD7ozZGrhyBgVn1MSs5eHXe1QPpD1vJgPNRQRzQ'),
};
export const METEORA_LABELS = { meteora: 'PumpMeteora', meteorav2: 'PumpMeteora V2' };

const METADATA_PROGRAM = new PublicKey('metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s');
// sha256("global:create_bonding_curve")[:8]
const CREATE_DISC = Uint8Array.from([94, 139, 158, 50, 69, 95, 8, 45]);

const enc = (s) => new TextEncoder().encode(s);
function pda(seeds, program) {
  return PublicKey.findProgramAddressSync(seeds, program)[0];
}
// borsh string: u32 LE length + utf8 bytes
function borshStr(s) {
  const d = enc(s);
  const out = new Uint8Array(4 + d.length);
  new DataView(out.buffer).setUint32(0, d.length, true);
  out.set(d, 4);
  return out;
}

// Build the create_bonding_curve instruction for `program`. `creator` = wallet pubkey,
// `mint` = the (fresh) mint pubkey. Reads team_wallet from the program's on-chain Config.
export async function buildCreateBondingCurveIx(connection, program, creator, mint, { name, symbol, uri }) {
  const config = pda([enc('config')], program);
  const globalVault = pda([enc('global')], program);
  const bondingCurve = pda([enc('bonding_curve'), mint.toBuffer()], program);
  const metadata = pda([enc('metadata'), METADATA_PROGRAM.toBuffer(), mint.toBuffer()], METADATA_PROGRAM);
  const globalAta = getAssociatedTokenAddressSync(mint, globalVault, true);

  const cfg = await connection.getAccountInfo(config);
  if (!cfg || !cfg.data || cfg.data.length < 8 + 96) {
    throw new Error('This program has no initialized config on-chain — cannot create a token here.');
  }
  const teamWallet = new PublicKey(cfg.data.slice(8 + 64, 8 + 96));

  const nB = borshStr(name), sB = borshStr(symbol), uB = borshStr(uri);
  const data = new Uint8Array(8 + nB.length + sB.length + uB.length);
  data.set(CREATE_DISC, 0);
  let o = 8;
  data.set(nB, o); o += nB.length;
  data.set(sB, o); o += sB.length;
  data.set(uB, o);

  const keys = [
    { pubkey: config, isSigner: false, isWritable: true },
    { pubkey: globalVault, isSigner: false, isWritable: true },
    { pubkey: creator, isSigner: true, isWritable: true },
    { pubkey: mint, isSigner: true, isWritable: true },
    { pubkey: bondingCurve, isSigner: false, isWritable: true },
    { pubkey: metadata, isSigner: false, isWritable: true },
    { pubkey: globalAta, isSigner: false, isWritable: true },
    { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
    { pubkey: SYSVAR_RENT_PUBKEY, isSigner: false, isWritable: false },
    { pubkey: TOKEN_PROGRAM_ID, isSigner: false, isWritable: false },
    { pubkey: ASSOCIATED_TOKEN_PROGRAM_ID, isSigner: false, isWritable: false },
    { pubkey: METADATA_PROGRAM, isSigner: false, isWritable: false },
    { pubkey: teamWallet, isSigner: false, isWritable: true },
  ];

  return new TransactionInstruction({ programId: program, keys, data: Buffer.from(data) });
}
