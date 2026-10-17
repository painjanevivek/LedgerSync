import { hasReservedTransferBalance, type TransferBalance } from "@/lib/api/transfers";
import { Money } from "@/ui/display/Money";

export function TransferReceiptBalance({balance,localized=false}:Readonly<{balance:TransferBalance;localized?:boolean}>) {
  return <><p>Posted ledger <Money localized={localized} currency={balance.currency} minorUnits={balance.posted_minor}/></p>{hasReservedTransferBalance(balance) ? <p>Available <Money localized={localized} currency={balance.currency} minorUnits={balance.available_minor!}/> · Reserved <Money localized={localized} currency={balance.currency} minorUnits={balance.reserved_minor!}/></p> : <small>This receipt does not confirm separate available and reserved values. Read the account for current funds.</small>}</>;
}
