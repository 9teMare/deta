#[test_only]
module aptos_data_network::user_vault_test {
    use std::vector;
    use aptos_framework::account;
    use aptos_data_network::UserVault;

    const USER1: address = @0x1;
    const USER2: address = @0x2;

    fun setup_user1(): signer {
        account::create_account_for_test(USER1)
    }

    fun setup_user2(): signer {
        account::create_account_for_test(USER2)
    }

    #[test]
    fun test_init_vault() {
        let user = setup_user1();
        UserVault::init(&user);

        // Verify vault was created
        let datasets = UserVault::get_datasets(USER1);
        assert!(vector::length(&datasets) == 0, 1);
        assert!(UserVault::get_dataset_count(USER1) == 0, 2);
    }

    #[test]
    fun test_add_dataset() {
        let user = setup_user1();
        UserVault::init(&user);

        UserVault::add_dataset(&user, 0);
        UserVault::add_dataset(&user, 1);
        UserVault::add_dataset(&user, 2);

        assert!(UserVault::get_dataset_count(USER1) == 3, 3);
        assert!(UserVault::has_dataset(USER1, 0) == true, 4);
        assert!(UserVault::has_dataset(USER1, 1) == true, 5);
        assert!(UserVault::has_dataset(USER1, 2) == true, 6);
        assert!(UserVault::has_dataset(USER1, 999) == false, 7);
    }

    #[test]
    fun test_remove_dataset() {
        let user = setup_user1();
        UserVault::init(&user);

        UserVault::add_dataset(&user, 0);
        UserVault::add_dataset(&user, 1);
        UserVault::add_dataset(&user, 2);

        assert!(UserVault::get_dataset_count(USER1) == 3, 8);

        // Remove middle dataset
        UserVault::remove_dataset(&user, 1);

        assert!(UserVault::get_dataset_count(USER1) == 2, 9);
        assert!(UserVault::has_dataset(USER1, 0) == true, 10);
        assert!(UserVault::has_dataset(USER1, 1) == false, 11);
        assert!(UserVault::has_dataset(USER1, 2) == true, 12);
    }

    #[test]
    fun test_duplicate_prevention() {
        let user = setup_user1();
        UserVault::init(&user);

        UserVault::add_dataset(&user, 0);
        UserVault::add_dataset(&user, 0); // Try to add duplicate

        // Should still be 1 (duplicate prevented)
        assert!(UserVault::get_dataset_count(USER1) == 1, 13);
    }

    #[test]
    fun test_get_datasets() {
        let user = setup_user1();
        UserVault::init(&user);

        UserVault::add_dataset(&user, 5);
        UserVault::add_dataset(&user, 10);
        UserVault::add_dataset(&user, 15);

        let datasets = UserVault::get_datasets(USER1);
        assert!(vector::length(&datasets) == 3, 14);
        assert!(*vector::borrow(&datasets, 0) == 5, 15);
        assert!(*vector::borrow(&datasets, 1) == 10, 16);
        assert!(*vector::borrow(&datasets, 2) == 15, 17);
    }

    #[test]
    fun test_nonexistent_vault() {
        // Test functions on non-existent vault
        let datasets = UserVault::get_datasets(USER2);
        assert!(vector::length(&datasets) == 0, 18);
        assert!(UserVault::get_dataset_count(USER2) == 0, 19);
        assert!(UserVault::has_dataset(USER2, 0) == false, 20);
    }
}

