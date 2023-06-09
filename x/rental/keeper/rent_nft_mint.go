package keeper

import (
	context "context"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	nft "github.com/cosmos/cosmos-sdk/x/nft"
	types "github.com/terramirum/mirumd/x/rental/types"
)

// RentNftMint implements types.MsgServer
func (k Keeper) RentNftMint(context context.Context, rentNftRequest *types.MsgMintRentRequest) (*types.MsgMintRentResponse, error) {
	ctx := sdk.UnwrapSDKContext(context)
	store := ctx.KVStore(k.storeKey)

	if !k.nftKeeper.HasClass(ctx, rentNftRequest.ClassId) {
		return nil, sdkerrors.Wrap(nft.ErrClassNotExists, rentNftRequest.ClassId)
	}

	if !k.nftKeeper.HasNFT(ctx, rentNftRequest.ClassId, rentNftRequest.NftId) {
		return nil, sdkerrors.Wrap(nft.ErrNFTExists, rentNftRequest.NftId)
	}

	nftOwner := k.nftKeeper.GetOwner(ctx, rentNftRequest.ClassId, rentNftRequest.NftId)
	if nftOwner.String() != rentNftRequest.ContractOwner {
		return nil, sdkerrors.Wrap(types.ErrNftOwnerCanRent, "")
	}

	// todo: get nft and check minimum duration unit and get duration between dates.

	params := k.GetParams(ctx)

	currentDate := getNowUtcAddMin(params.RentMinStartUnit)
	if rentNftRequest.StartDate < currentDate {
		return nil, sdkerrors.Wrap(types.ErrNftRentMinStartDate, fmt.Sprintf("Start Date: %d", currentDate))
	}

	if rentNftRequest.StartDate >= rentNftRequest.EndDate {
		return nil, sdkerrors.Wrap(types.ErrStartDateBiggerEndDate, "")
	}

	nftRents := k.GetSessionIdsOfNft(ctx, rentNftRequest.ClassId, rentNftRequest.NftId)
	k.clearOldSession(ctx, rentNftRequest.ClassId, rentNftRequest.NftId, nftRents)

	if k.hasAvaliableSession(nftRents, rentNftRequest) {
		sessionId, nftRent := k.saveSessionOfNft(store, rentNftRequest)

		rentersKey := getStoreWithKey(KeyRentSessionId, rentNftRequest.Renter, rentNftRequest.ClassId, rentNftRequest.NftId, sessionId)
		bz := k.cdc.MustMarshal(nftRent)
		store.Set(rentersKey, bz)
	} else {
		return nil, sdkerrors.Wrap(types.ErrNftRentNotAvaliableDate, fmt.Sprintf("Rented Dates: \n %s", getRentedDates(rentNftRequest, nftRents)))
	}

	// ilgili tarihler icin mint eden adress başka kişiye yetki verebilir.
	// Contract owner aynı tarihte birden fazla kişiye yetki verebilir.
	// Bu kişiler birbirile yeni bir session id ile baglanacak
	// herbir tarih yeni bir session id alacak.
	// session id başlangıc ve bitiş toplamı olabilir
	// session id aynı zamanda yetkili kişileri icermekte olacak

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeRentNft,
		sdk.NewAttribute(types.AttributeKeyNftRentReciever, rentNftRequest.Renter),
		sdk.NewAttribute(types.AttributeKeyNftRentStart, fmt.Sprintf("%d", rentNftRequest.StartDate)),
		sdk.NewAttribute(types.AttributeKeyNftRentEnd, fmt.Sprintf("%d", rentNftRequest.EndDate)),
		sdk.NewAttribute(types.AttributeKeyClassId, rentNftRequest.ClassId),
		sdk.NewAttribute(types.AttributeKeyNftId, rentNftRequest.NftId),
	))

	return &types.MsgMintRentResponse{}, nil
}

func getRentedDates(rentNftRequest *types.MsgMintRentRequest, nftRents []*types.NftRent) string {
	var builder strings.Builder
	for _, v := range nftRents {
		builder.WriteString(fmt.Sprintf("Start: %d End: %d \n", v.StartDate, v.EndDate))
	}
	return builder.String()
}

func (k Keeper) saveSessionOfNft(store sdk.KVStore, rentNftRequest *types.MsgMintRentRequest) (sessionId string, nftRent *types.NftRent) {
	sessionId = fmt.Sprintf("%d", rentNftRequest.StartDate)
	keySession := getStoreWithKey(KeyRentDates, rentNftRequest.ClassId, rentNftRequest.NftId, sessionId)
	nftRent = &types.NftRent{
		StartDate: rentNftRequest.StartDate,
		EndDate:   rentNftRequest.EndDate,
		SessionId: sessionId,
	}
	bz := k.cdc.MustMarshal(nftRent)
	store.Set(keySession, bz)

	// storing owner to KeyRentDates.
	keySession = getStoreWithKey(KeyRentDatesOwner, rentNftRequest.ClassId, rentNftRequest.NftId, sessionId, rentNftRequest.Renter)
	store.Set(keySession, []byte(rentNftRequest.Renter))

	return sessionId, nftRent
}

func (k Keeper) hasAvaliableSession(nftRents []*types.NftRent, rentNftRequest *types.MsgMintRentRequest) bool {
	for _, v := range nftRents {
		if v.StartDate <= rentNftRequest.StartDate && v.EndDate >= rentNftRequest.StartDate {
			return false
		}
		if v.StartDate <= rentNftRequest.EndDate && v.EndDate >= rentNftRequest.EndDate {
			return false
		}
	}
	return true
}

func (k Keeper) GetSessionIdsOfNft(ctx sdk.Context, classId, nftId string) (nftRents []*types.NftRent) {
	store := ctx.KVStore(k.storeKey)
	key := getStoreWithKey(KeyRentDates, classId, nftId)
	allSessionStore := prefix.NewStore(store, key)
	iterator := allSessionStore.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var nftRent types.NftRent
		k.cdc.MustUnmarshal(iterator.Value(), &nftRent)
		nftRents = append(nftRents, &nftRent)
	}
	return nftRents
}

// clear old sessions.
func (k Keeper) clearOldSession(ctx sdk.Context, classId, nftId string, nftRents []*types.NftRent) {
	currentDate := getNowUtc()
	store := ctx.KVStore(k.storeKey)
	for _, v := range nftRents {
		if v.EndDate < currentDate {
			sessionId := fmt.Sprintf("%d", v.StartDate)
			keySession := getStoreWithKey(KeyRentDates, classId, nftId, sessionId)
			store.Delete(keySession)
			k.clearKeyRentDatesOwner(ctx, classId, nftId, sessionId)
		}
	}
}

func (k Keeper) clearKeyRentDatesOwner(ctx sdk.Context, classId, nftId, sessionId string) {
	store := ctx.KVStore(k.storeKey)

	key := getStoreWithKey(KeyRentDatesOwner, classId, nftId, sessionId)
	allSessionStore := prefix.NewStore(store, key)
	iterator := allSessionStore.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		keys := getParsedStoreKey(iterator.Key())
		rentersKey := getStoreWithKey(KeyRentSessionId, keys[1], classId, nftId, sessionId)
		store.Delete(rentersKey)

		store.Delete(iterator.Key())
	}
}
