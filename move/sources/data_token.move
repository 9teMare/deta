module datax::data_token {
    use aptos_framework::managed_coin;
    use aptos_framework::coin;

    /// Resource that holds the mint capability.
    struct DataToken has key {}

    const DECIMALS: u8 = 6;
    const NAME: vector<u8> = b"Data Token";
    const SYMBOL: vector<u8> = b"DTN";

    /// Initializes the DataToken. Must be called once by contract owner.
    public entry fun init(owner: &signer) {
        // Initialize the coin (mint capability is stored automatically)
        managed_coin::initialize<DataToken>(
            owner,
            NAME,
            SYMBOL,
            DECIMALS,
            false // monitor_supply
        );
    }


    /// Register a user to receive DTN
    public entry fun register(account: &signer) {
        coin::register<DataToken>(account);
    }

    /// Mint DTN to the owner or data contributors.
    public entry fun mint(owner: &signer, recipient: address, amount: u64) {
        managed_coin::mint<DataToken>(owner, recipient, amount);
    }
}

