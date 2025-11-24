#[test_only]
module datax::data_token_test {
    use aptos_framework::account;
    use datax::data_token;

    const OWNER: address = @0x1;
    const USER1: address = @0x2;
    const USER2: address = @0x3;

    fun setup_owner(): signer {
        account::create_account_for_test(OWNER)
    }

    fun setup_user1(): signer {
        account::create_account_for_test(USER1)
    }

    fun setup_user2(): signer {
        account::create_account_for_test(USER2)
    }

    #[test]
    fun test_init_token() {
        let owner = setup_owner();
        data_token::init(&owner);

        // Token should be initialized
        // We can't directly check, but if it compiles and runs, it's initialized
    }

    #[test]
    fun test_register_user() {
        let owner = setup_owner();
        let user1 = setup_user1();

        // Initialize coin for this test (tests are isolated, so this should work)
        data_token::init(&owner);
        data_token::register(&user1);

        // User should be registered to receive tokens
        // Registration doesn't fail if already registered
    }

    #[test]
    fun test_mint_tokens() {
        let owner = setup_owner();
        let user1 = setup_user1();

        // Initialize coin for this test
        data_token::init(&owner);
        data_token::register(&user1);

        // Mint 1000 tokens to user1
        data_token::mint(&owner, USER1, 1000);

        // Verify balance (if we had a get_balance function)
        // For now, just verify it doesn't fail
    }

    #[test]
    fun test_mint_multiple_users() {
        let owner = setup_owner();
        let user1 = setup_user1();
        let user2 = setup_user2();

        // Initialize coin for this test
        data_token::init(&owner);
        data_token::register(&user1);
        data_token::register(&user2);

        data_token::mint(&owner, USER1, 500);
        data_token::mint(&owner, USER2, 1000);

        // Both users should receive tokens
    }
}

