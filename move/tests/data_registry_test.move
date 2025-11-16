#[test_only]
module datax::data_registry_test {
    use aptos_framework::account;
    use aptos_framework::timestamp;
    use datax::data_registry;

    const ADMIN: address = @0x1;
    const USER1: address = @0x2;
    const USER2: address = @0x3;

    fun setup_admin(): signer {
        account::create_account_for_test(ADMIN)
    }

    fun setup_user1(): signer {
        account::create_account_for_test(USER1)
    }

    fun setup_user2(): signer {
        account::create_account_for_test(USER2)
    }

    fun setup_timestamp(aptos_framework: &signer) {
        timestamp::set_time_has_started_for_testing(aptos_framework);
    }

    #[test]
    fun test_init_data_store() {
        let user = setup_user1();
        data_registry::init(&user);

        // Verify store was created
        assert!(data_registry::get_dataset_count(USER1) == 0, 1);
    }

    #[test]
    fun test_submit_data() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user = setup_user1();
        data_registry::init(&user);

        let data_hash = b"test_hash_123";
        let metadata = b"test_metadata";

        data_registry::submit_data(&user, data_hash, metadata);

        // Verify dataset was created
        assert!(data_registry::get_dataset_count(USER1) == 1, 2);

        // Verify dataset info
        let (hash, meta, created_at, is_active) = data_registry::get_dataset(USER1, 0);
        assert!(hash == data_hash, 3);
        assert!(meta == metadata, 4);
        assert!(is_active == true, 5);
    }

    #[test]
    fun test_submit_multiple_datasets() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user = setup_user1();
        data_registry::init(&user);

        data_registry::submit_data(&user, b"hash1", b"meta1");
        data_registry::submit_data(&user, b"hash2", b"meta2");
        data_registry::submit_data(&user, b"hash3", b"meta3");

        assert!(data_registry::get_dataset_count(USER1) == 3, 6);

        let (hash, _, _, _) = data_registry::get_dataset(USER1, 0);
        assert!(hash == b"hash1", 7);

        let (hash, _, _, _) = data_registry::get_dataset(USER1, 1);
        assert!(hash == b"hash2", 8);

        let (hash, _, _, _) = data_registry::get_dataset(USER1, 2);
        assert!(hash == b"hash3", 9);
    }

    #[test]
    fun test_delete_dataset() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user = setup_user1();
        data_registry::init(&user);

        data_registry::submit_data(&user, b"hash1", b"meta1");
        data_registry::submit_data(&user, b"hash2", b"meta2");

        assert!(data_registry::get_dataset_count(USER1) == 2, 10);

        // Delete first dataset
        data_registry::delete_dataset(&user, 0);

        // Verify it's marked as inactive
        let (_, _, _, is_active) = data_registry::get_dataset(USER1, 0);
        assert!(is_active == false, 11);

        // Count should still be 2 (dataset exists but inactive)
        assert!(data_registry::get_dataset_count(USER1) == 2, 12);
    }

    #[test]
    #[expected_failure(abort_code = 3, location = data_registry)]
    fun test_delete_dataset_not_owner() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user1 = setup_user1();
        let user2 = setup_user2();

        data_registry::init(&user1);
        data_registry::submit_data(&user1, b"hash1", b"meta1");

        // User2 tries to delete user1's dataset - should fail
        data_registry::delete_dataset(&user2, 0);
    }

    #[test]
    fun test_auto_initialization() {
        let aptos_framework = account::create_account_for_test(@aptos_framework);
        setup_timestamp(&aptos_framework);
        let user = setup_user1();

        // Submit data without explicit init - should auto-initialize
        data_registry::submit_data(&user, b"hash1", b"meta1");

        assert!(data_registry::get_dataset_count(USER1) == 1, 13);
    }

    #[test]
    #[expected_failure(abort_code = 2, location = data_registry)]
    fun test_get_nonexistent_dataset() {
        let user = setup_user1();
        data_registry::init(&user);

        // Try to get dataset that doesn't exist
        data_registry::get_dataset(USER1, 999);
    }
}

