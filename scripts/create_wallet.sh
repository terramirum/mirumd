# this is secret. Please chage it when intalling full node.
# should be changed
PASSWORD=${PASSWORD:-12345678} 

echo "Wallet create is started.  "

if ! mirumd keys show validator; then
   (echo "$PASSWORD"; echo "$PASSWORD") | mirumd keys add validator
fi

echo "!!!!!!!! store your mnemonic words to backup or import it any keplr wallet. !!!!!!!!" 
echo "transfer some coin from anywhere like stock exchange r keplr wallet" 
echo "Check your balance with below command. Make sure node fully sychronize with block number."
echo "Use below command to check your balance"
echo "mirumd query bank balances <wallet address>"
echo "Wallet balance could be bigger then you expect. there is 6 decimal precision. Devide it to 1000000 to get exact balance."
echo "to be a validator, execute create_validator.sh" 