import { Transaction, VersionedTransaction, PublicKey } from '@solana/web3.js';
import { Buffer } from 'buffer';

export const SWAP_BUY = 1;
export const SWAP_SELL = 2;
export const CHAIN_ID = 100000;

/**
 * Build (backend), sign (wallet) and submit (devnet RPC) a market order.
 * Routes the send through the app's own connection so it doesn't depend on the
 * wallet's selected network for submission.
 */
export async function submitMarketOrder({
  connection,
  publicKey,
  signTransaction,
  sendTransaction,
  tokenCa,
  swapType,
  amountIn,
  doubleOut = false,
  trailingPercent = 0,
  autoSlippage = true,
}) {
  if (!publicKey) throw new Error('Connect your wallet first');

  // 1) ask the backend to build the unsigned transaction
  const res = await fetch('/v1/trade/create_market_order', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      chain_id: CHAIN_ID,
      token_ca: tokenCa,
      swap_type: swapType,
      amount_in: String(amountIn),
      double_out: doubleOut,
      // buy only: attach a trailing stop for the fill (custodial, like double_out)
      ...(trailingPercent > 0 ? { trailing_percent: trailingPercent } : {}),
      user_wallet_address: publicKey.toString(),
      // Auto slippage: on a slippage failure the backend escalates the tier
      // (base → 45% → 70%) and rebuilds the swap instead of failing. Only
      // effective on server-signed sends (double-out / trailing legs and
      // triggered orders) — for the unsigned-tx path the flag is just stored.
      // NOTE: base slippage itself is still fixed server-side (50% market /
      // 10% limit); the proto has no numeric slippage field yet.
      is_auto_slippage: autoSlippage,
    }),
  });
  const data = await res.json();
  if (data.code && data.code !== 10000 && data.code !== 0) {
    throw new Error(data.message || `Backend error ${data.code}`);
  }
  const b64 = data.data?.txHash || data.txHash || data.tx_hash;
  if (!b64) throw new Error('No transaction returned by server');
  const orderId = data.data?.orderId ?? data.orderId ?? data.order_id;

  // Every buy — including double-out/trailing-stop-attached ones — is an
  // unsigned tx the user signs here, so the tokens land in their own wallet.
  // Only the auto-created follow-up sell leg (2x sell / drawdown sell) is
  // executed server-side later, once it triggers.
  //
  // The backend has no other way to learn what happens next — it just handed
  // over an unsigned tx — so whichever way this goes (signed+confirmed, or
  // rejected/failed), report it back via confirm_market_order. Without that
  // the order sits at "Triggered" (Proc) forever: invisible to the ticker
  // that would otherwise finalize it and create any attached follow-up leg.
  const reportOutcome = async (payload) => {
    if (!orderId) return; // server-signed path never returns one — nothing to report
    try {
      await fetch('/v1/trade/confirm_market_order', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ order_id: orderId, ...payload }),
      });
    } catch (e) {
      console.error('confirm_market_order failed:', e);
    }
  };

  try {
    // 2) decode (legacy or versioned)
    const buf = Buffer.from(b64, 'base64');
    let tx;
    let versioned = false;
    try {
      tx = Transaction.from(buf);
    } catch {
      tx = VersionedTransaction.deserialize(buf);
      versioned = true;
    }

    // 3) refresh blockhash for legacy txs, sign, submit via the app's devnet RPC
    const latest = await connection.getLatestBlockhash('finalized');
    if (!versioned) {
      tx.recentBlockhash = latest.blockhash;
      tx.feePayer = publicKey;
    }

    let signature;
    if (signTransaction) {
      const signed = await signTransaction(tx);
      signature = await connection.sendRawTransaction(signed.serialize(), {
        skipPreflight: false,
        maxRetries: 3,
      });
    } else {
      signature = await sendTransaction(tx, connection);
    }

    await connection.confirmTransaction(
      { signature, blockhash: latest.blockhash, lastValidBlockHeight: latest.lastValidBlockHeight },
      'confirmed'
    );
    await reportOutcome({ tx_hash: signature });
    return signature;
  } catch (e) {
    await reportOutcome({ error: String(e?.message || e).slice(0, 500) });
    throw e;
  }
}

/** Fetch the connected wallet's balance (UI amount) of a given mint. */
export async function getTokenBalance(connection, owner, mint) {
  try {
    const resp = await connection.getParsedTokenAccountsByOwner(new PublicKey(owner), {
      mint: new PublicKey(mint),
    });
    let ui = 0;
    for (const { account } of resp.value) {
      ui += account.data.parsed.info.tokenAmount.uiAmount || 0;
    }
    return ui;
  } catch {
    return 0;
  }
}

export function shortAddr(a, n = 4) {
  if (!a) return '';
  return `${a.slice(0, n)}…${a.slice(-n)}`;
}

/* ---------------- Limit orders ---------------- */

// trade_order.status values (see trade.proto OrderStatus)
export const ORDER_STATUS = {
  1: { label: 'Waiting', tone: 'warn' },
  2: { label: 'Triggered', tone: 'accent' },
  3: { label: 'On chain', tone: 'accent' },
  4: { label: 'Failed', tone: 'down' },
  5: { label: 'Filled', tone: 'up' },
  6: { label: 'Cancelled', tone: 'muted' },
  7: { label: 'Timeout', tone: 'down' },
};

function unwrap(data) {
  if (data.code && data.code !== 10000 && data.code !== 0) {
    throw new Error(data.message || `Backend error ${data.code}`);
  }
  return data.data ?? data;
}

async function post(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return unwrap(await res.json());
}

/**
 * Records a newly-created token or pool against the connected wallet so it
 * shows up on the Portfolio page. Best-effort: callers should not fail the
 * whole creation flow if this call fails (the on-chain asset already exists
 * either way — only its Portfolio listing would be missing).
 */
export async function recordUserAsset(payload) {
  return post('/v1/market/record_user_asset', { chain_id: 100000, ...payload });
}

/**
 * Place a limit order. The backend stores it (MySQL + Redis trigger list) and
 * executes it as a market order when an on-chain trade brings the price across
 * the limit. Buy: amount = SOL to spend. Sell: amount = tokens to sell.
 * Returns the order id.
 */
export async function submitLimitOrder({
  tokenCa,
  swapType,
  amount,
  priceUsd,
  doubleOut = false,
  autoSlippage = true,
}) {
  const d = await post('/v1/trade/create_limit_order', {
    chain_id: CHAIN_ID,
    token_ca: tokenCa,
    swap_type: swapType,
    amount: String(amount),
    price_usd: String(priceUsd),
    double_out: doubleOut,
    is_auto_slippage: autoSlippage,
  });
  return Number(d.orderId ?? d.order_id);
}

/**
 * Place a trailing stop (移动止盈止损) sell order. The backend anchors at the
 * current price and market-sells when the price falls trailingPercent% from
 * the highest price seen after placement — the trigger line only ratchets up.
 * amount = tokens to sell. Returns the order id.
 */
export async function submitTrailingStop({ tokenCa, amount, trailingPercent, autoSlippage = true }) {
  const d = await post('/v1/trade/create_trailing_stop', {
    chain_id: CHAIN_ID,
    token_ca: tokenCa,
    amount: String(amount),
    trailing_percent: trailingPercent,
    is_auto_slippage: autoSlippage,
  });
  return Number(d.orderId ?? d.order_id);
}

/** Open (non-terminal) orders for a token. protojson emits int64 as strings — normalize. */
export async function fetchCurrentOrders(tokenCa) {
  const d = await post('/v1/trade/query_current_orders', {
    chain_id: CHAIN_ID,
    token_ca: tokenCa,
    page_no: 1,
    page_size: 50,
  });
  return (d.list || []).map((o) => ({
    id: Number(o.id),
    swapType: Number(o.swapType ?? o.swap_type),
    tradeType: Number(o.tradeType ?? o.trade_type),
    amount: o.amount,
    priceUsd: Number(o.priceUsd ?? o.price_usd),
    status: Number(o.status),
    createTime: Number(o.createTime ?? o.create_time),
    doubleOut: Number(o.doubleOut ?? o.double_out ?? 0),
    trailingPercent: Number(o.trailingPercent ?? o.trailing_percent ?? 0),
  }));
}

export async function cancelOrder(orderId) {
  await post('/v1/trade/cancel_order', { order_id: orderId });
}
