package lnd

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/lncm/invoicer/common"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
	"io/ioutil"
	"time"
)

const (
	ClientName = "lnd"

	DefaultHostname = "localhost"
	DefaultPort     = 10009
	DefaultTls      = "./tls.cert"
	DefaultInvoice  = "./invoice.macaroon"
	DefaultReadOnly = "./readonly.macaroon"
)

type Lnd struct {
	invoiceClient  lnrpc.LightningClient
	readOnlyClient lnrpc.LightningClient
}

func (lnd Lnd) NewInvoice(ctx context.Context, amount int64, desc string) (invoice, hash string, err error) {
	inv, err := lnd.invoiceClient.AddInvoice(ctx, &lnrpc.Invoice{
		Memo:   desc,
		Value:  int64(amount),
		Expiry: common.DefaultInvoiceExpiry,
	})
	if err != nil {
		return
	}

	return inv.PaymentRequest, hex.EncodeToString(inv.RHash), nil
}

// TODO: do not resubscribe on each request
func (lnd Lnd) StatusWait(ctx context.Context, hash string) (s common.Status, err error) {
	invSub, err := lnd.invoiceClient.SubscribeInvoices(ctx, &lnrpc.InvoiceSubscription{})
	if err != nil {
		return
	}

	for {
		var inv *lnrpc.Invoice
		inv, err = invSub.Recv()
		if err != nil {
			return
		}

		if hash == hex.EncodeToString(inv.RHash) {
			return common.Status{
				Ts:      inv.CreationDate,
				Settled: inv.Settled,
				Expiry:  inv.Expiry,
				Value:   inv.Value,
			}, nil
		}
	}
}

func (lnd Lnd) Status(ctx context.Context, hash string) (s common.Status, err error) {
	invId, err := hex.DecodeString(hash)
	if err != nil {
		return
	}

	inv, err := lnd.invoiceClient.LookupInvoice(ctx, &lnrpc.PaymentHash{RHash: invId})
	if err != nil {
		return
	}

	return common.Status{
		Ts:      inv.CreationDate,
		Settled: inv.Settled,
		Expiry:  inv.Expiry,
		Value:   inv.Value,
	}, nil
}

func (lnd Lnd) Address(ctx context.Context, bech32 bool) (address string, err error) {
	addrType := lnrpc.NewAddressRequest_NESTED_PUBKEY_HASH
	if bech32 {
		addrType = lnrpc.NewAddressRequest_WITNESS_PUBKEY_HASH
	}

	addrResp, err := lnd.invoiceClient.NewAddress(ctx, &lnrpc.NewAddressRequest{
		Type: addrType,
	})
	if err != nil {
		return
	}

	return addrResp.Address, nil
}

func (lnd Lnd) Info(ctx context.Context) (info common.Info, err error) {
	i, err := lnd.readOnlyClient.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return
	}

	return common.Info{Uris: i.Uris}, nil
}

func (lnd Lnd) History(ctx context.Context) (invoices common.Invoices, err error) {
	list, err := lnd.readOnlyClient.ListInvoices(ctx, &lnrpc.ListInvoiceRequest{
		NumMaxInvoices: 100,
		Reversed:       true,
	})
	if err != nil {
		return
	}

	for _, inv := range list.Invoices {
		invoices = append(invoices, common.Invoice{
			Description: inv.Memo,
			Amount:      inv.Value,
			Paid:        inv.Settled,
			PaidAt:      inv.SettleDate,
			Expired:     inv.CreationDate+inv.Expiry < time.Now().Unix(),
			NewPayment: common.NewPayment{
				Bolt11:    inv.PaymentRequest,
				Hash:      hex.EncodeToString(inv.RHash),
				CreatedAt: inv.CreationDate,
				Expiry:    inv.Expiry,
			},
		})
	}

	// TODO: optionally reverse order(?)

	return
}

func getClient(creds credentials.TransportCredentials, fullHostname, file string) lnrpc.LightningClient {
	macaroonBytes, err := ioutil.ReadFile(file)
	if err != nil {
		panic(fmt.Sprintln("Cannot read macaroon file", err))
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		panic(fmt.Sprintln("Cannot unmarshal macaroon", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := grpc.DialContext(ctx, fullHostname, []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	}...)
	if err != nil {
		panic(errors.Wrapf(err, "unable to connect to %s", fullHostname))
	}

	return lnrpc.NewLightningClient(connection)
}

func New(conf common.Lnd) Lnd {
	if conf.Host == "" {
		conf.Host = DefaultHostname
	}

	if conf.Port == 0 {
		conf.Port = DefaultPort
	}

	if conf.Tls == "" {
		conf.Tls = DefaultTls
	}
	conf.Tls = common.CleanAndExpandPath(conf.Tls)

	if conf.Macaroons.Invoice == "" {
		conf.Macaroons.Invoice = DefaultInvoice
	}
	conf.Macaroons.Invoice = common.CleanAndExpandPath(conf.Macaroons.Invoice)

	if conf.Macaroons.ReadOnly == "" {
		conf.Macaroons.ReadOnly = DefaultReadOnly
	}
	conf.Macaroons.ReadOnly = common.CleanAndExpandPath(conf.Macaroons.ReadOnly)

	creds, err := credentials.NewClientTLSFromFile(conf.Tls, conf.Host)
	if err != nil {
		panic(err)
	}

	hostname := fmt.Sprintf("%s:%d", conf.Host, conf.Port)

	return Lnd{
		getClient(creds, hostname, conf.Macaroons.Invoice),
		getClient(creds, hostname, conf.Macaroons.ReadOnly),
	}
}
