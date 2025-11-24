#[test_only]
module datax::integration_test {
    use std::vector;
    use aptos_framework::account;
    use aptos_framework::timestamp;
    use datax::data_registry;
    use datax::data_token;
    use datax::user_vault;
    use datax::access_control;

    const USER1: address = @0x1;
    const USER2: address = @0x2;
    const DATA_BUYER: address = @0x3;

    fun setup_user1(): signer {
        account::create_account_for_test(USER1)
    }

    fun setup_user2(): signer {
        account::create_account_for_test(USER2)
    }

    fun setup_buyer(): signer {
        account::create_account_for_test(DATA_BUYER)
    }

    fun setup_timestamp(aptos_framework: &signer) {
        timestamp::set_time_has_started_for_testing(aptos_framework);
    }

    #[test]
    fun test_full_workflow() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user1 = setup_user1();
        let _user2 = setup_user2();
        let _buyer = setup_buyer();

        // 1. Initialize token (tests are isolated, so this should work)
        data_token::init(&user1);

        // 2. User1 submits data (with encryption metadata)
        let empty_encryption = vector::empty<u8>();
        data_registry::submit_data(&user1, b"data_hash_1", b"metadata_1", empty_encryption, empty_encryption);
        data_registry::submit_data(&user1, b"data_hash_2", b"metadata_2", empty_encryption, empty_encryption);

        // 3. Verify data is stored
        assert!(data_registry::get_dataset_count(USER1) == 2, 1);
        assert!(user_vault::get_dataset_count(USER1) == 2, 2);

        // 4. User1 grants access to buyer
        let current_time = timestamp::now_seconds();
        let expires_at = current_time + 86400; // 24 hours
        access_control::grant_access(&user1, 0, DATA_BUYER, expires_at);

        // 5. Verify access
        assert!(
            access_control::has_access(USER1, 0, DATA_BUYER) == true,
            3
        );
        assert!(
            access_control::has_access(USER1, 1, DATA_BUYER) == false,
            4
        );

        // 6. User1 receives token reward for data contribution
        data_token::register(&user1);
        data_token::mint(&user1, USER1, 1000);

        // 7. User1 revokes access
        access_control::revoke_access(&user1, 0, DATA_BUYER);
        assert!(
            access_control::has_access(USER1, 0, DATA_BUYER) == false,
            5
        );

        // 8. User1 deletes their data (full control)
        data_registry::delete_dataset(&user1, 0);
        let (_, _, _, _, _, is_active) = data_registry::get_dataset(USER1, 0);
        assert!(is_active == false, 6);
        assert!(user_vault::get_dataset_count(USER1) == 1, 7); // Removed from vault
    }

    #[test]
    fun test_multiple_users_data_network() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user1 = setup_user1();
        let user2 = setup_user2();
        let _buyer = setup_buyer();

        // Initialize token (tests are isolated, so this should work)
        data_token::init(&user1);

        // Both users contribute data
        let empty_encryption = vector::empty<u8>();
        data_registry::submit_data(&user1, b"user1_data", b"user1_meta", empty_encryption, empty_encryption);
        data_registry::submit_data(&user2, b"user2_data", b"user2_meta", empty_encryption, empty_encryption);

        // Verify each user's data is separate
        assert!(data_registry::get_dataset_count(USER1) == 1, 8);
        assert!(data_registry::get_dataset_count(USER2) == 1, 9);
        assert!(user_vault::get_dataset_count(USER1) == 1, 10);
        assert!(user_vault::get_dataset_count(USER2) == 1, 11);

        // Each user controls their own access
        let current_time = timestamp::now_seconds();
        access_control::grant_access(&user1, 0, DATA_BUYER, current_time + 3600);
        access_control::grant_access(&user2, 0, DATA_BUYER, current_time + 3600);

        assert!(
            access_control::has_access(USER1, 0, DATA_BUYER) == true,
            12
        );
        assert!(
            access_control::has_access(USER2, 0, DATA_BUYER) == true,
            13
        );

        // Users can independently revoke
        access_control::revoke_access(&user1, 0, DATA_BUYER);
        assert!(
            access_control::has_access(USER1, 0, DATA_BUYER) == false,
            14
        );
        assert!(
            access_control::has_access(USER2, 0, DATA_BUYER) == true,
            15
        ); // User2's access still valid
    }

    #[test]
    fun test_user_data_sovereignty() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user1 = setup_user1();
        let _user2 = setup_user2();

        // User1 submits data
        let empty_encryption = vector::empty<u8>();
        data_registry::submit_data(&user1, b"sensitive_data", b"private_meta", empty_encryption, empty_encryption);

        // User2 cannot access without permission
        assert!(
            access_control::has_access(USER1, 0, USER2) == false,
            16
        );

        // User1 has full control - can delete
        data_registry::delete_dataset(&user1, 0);
        let (_, _, _, _, _, is_active) = data_registry::get_dataset(USER1, 0);
        assert!(is_active == false, 17);

        // User1's vault is updated
        assert!(user_vault::has_dataset(USER1, 0) == false, 18);
    }
}

