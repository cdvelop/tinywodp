package tinywodp

import (
	. "github.com/cdvelop/tinystring"
	"sync"
	"testing"
)

// TestJsonRaceCondition tests for race conditions in JSON encode/decode operations
// with nested structures to ensure data integrity under concurrent access
func TestJsonRaceCondition(t *testing.T) {
	const numGoroutines = 50
	const numOperations = 100

	// Create test data with nested structures
	testUser := GenerateComplexTestData(1)[0]

	// Verify initial data integrity
	expectedID := testUser.ID
	expectedFirstName := testUser.Profile.FirstName
	expectedPhoneID := testUser.Profile.PhoneNumbers[0].ID
	expectedAddressID := testUser.Profile.Addresses[0].ID
	expectedLatitude := testUser.Profile.Addresses[0].Coordinates.Latitude

	var wg sync.WaitGroup
	errors := make([]error, numGoroutines)

	// Launch multiple goroutines performing concurrent JSON operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Test encode/decode cycle
				jsonData, err := Convert(&testUser).JsonEncode()
				if err != nil {
					errors[goroutineID] = err
					return
				}

				// Decode back to struct
				var decodedUser ComplexUser
				err = Convert(string(jsonData)).JsonDecode(&decodedUser)
				if err != nil {
					errors[goroutineID] = err
					return
				}

				// Verify data integrity - check that nested data hasn't been corrupted
				if decodedUser.ID != expectedID {
					t.Errorf("Goroutine %d, iteration %d: ID corruption detected. Expected: %s, Got: %s",
						goroutineID, j, expectedID, decodedUser.ID)
					return
				}

				if decodedUser.Profile.FirstName != expectedFirstName {
					t.Errorf("Goroutine %d, iteration %d: FirstName corruption detected. Expected: %s, Got: %s",
						goroutineID, j, expectedFirstName, decodedUser.Profile.FirstName)
					return
				}

				if len(decodedUser.Profile.PhoneNumbers) == 0 {
					t.Errorf("Goroutine %d, iteration %d: PhoneNumbers slice corrupted - empty", goroutineID, j)
					return
				}

				if decodedUser.Profile.PhoneNumbers[0].ID != expectedPhoneID {
					t.Errorf("Goroutine %d, iteration %d: PhoneNumber ID corruption detected. Expected: %s, Got: %s",
						goroutineID, j, expectedPhoneID, decodedUser.Profile.PhoneNumbers[0].ID)
					return
				}

				if len(decodedUser.Profile.Addresses) == 0 {
					t.Errorf("Goroutine %d, iteration %d: Addresses slice corrupted - empty", goroutineID, j)
					return
				}

				if decodedUser.Profile.Addresses[0].ID != expectedAddressID {
					t.Errorf("Goroutine %d, iteration %d: Address ID corruption detected. Expected: %s, Got: %s",
						goroutineID, j, expectedAddressID, decodedUser.Profile.Addresses[0].ID)
					return
				}

				if decodedUser.Profile.Addresses[0].Coordinates == nil {
					t.Errorf("Goroutine %d, iteration %d: Coordinates pointer corrupted - nil", goroutineID, j)
					return
				}

				if decodedUser.Profile.Addresses[0].Coordinates.Latitude != expectedLatitude {
					t.Errorf("Goroutine %d, iteration %d: Latitude corruption detected. Expected: %f, Got: %f",
						goroutineID, j, expectedLatitude, decodedUser.Profile.Addresses[0].Coordinates.Latitude)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for any errors from goroutines
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d failed with error: %v", i, err)
		}
	}
}

// TestJsonRaceConditionSimpleStructs tests race conditions with simpler structures
func TestJsonRaceConditionSimpleStructs(t *testing.T) {
	const numGoroutines = 30
	const numOperations = 50

	testPerson := GenerateSimplePersonData()
	expectedName := testPerson.Name
	expectedPhone := testPerson.Phone
	expectedAddressCount := len(testPerson.Addresses)

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Encode
				jsonData, err := Convert(&testPerson).JsonEncode()
				if err != nil {
					errorChan <- err
					return
				}

				// Decode
				var decodedPerson Person
				err = Convert(string(jsonData)).JsonDecode(&decodedPerson)
				if err != nil {
					errorChan <- err
					return
				}

				// Verify integrity
				if decodedPerson.Name != expectedName {
					t.Errorf("Goroutine %d: Name corruption. Expected: %s, Got: %s",
						goroutineID, expectedName, decodedPerson.Name)
				}

				if decodedPerson.Phone != expectedPhone {
					t.Errorf("Goroutine %d: Phone corruption. Expected: %s, Got: %s",
						goroutineID, expectedPhone, decodedPerson.Phone)
				}

				if len(decodedPerson.Addresses) != expectedAddressCount {
					t.Errorf("Goroutine %d: Address count corruption. Expected: %d, Got: %d",
						goroutineID, expectedAddressCount, len(decodedPerson.Addresses))
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			t.Errorf("Race condition test failed with error: %v", err)
		}
	}
}

// TestJsonRaceConditionSliceOperations tests race conditions specifically on slice operations
func TestJsonRaceConditionSliceOperations(t *testing.T) {
	const numGoroutines = 20
	const numOperations = 30

	// Create test data with multiple elements
	testUsers := GenerateComplexTestData(5)

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Encode slice
				jsonData, err := Convert(&testUsers).JsonEncode()
				if err != nil {
					errorChan <- err
					return
				}

				// Decode slice
				var decodedUsers []ComplexUser
				err = Convert(string(jsonData)).JsonDecode(&decodedUsers)
				if err != nil {
					errorChan <- err
					return
				}

				// Verify slice integrity
				if len(decodedUsers) != len(testUsers) {
					t.Errorf("Goroutine %d: Slice length corruption. Expected: %d, Got: %d",
						goroutineID, len(testUsers), len(decodedUsers))
					continue
				}

				// Check each element for data corruption
				for k, user := range decodedUsers {
					expectedUser := testUsers[k]

					if user.ID != expectedUser.ID {
						t.Errorf("Goroutine %d, user %d: ID corruption. Expected: %s, Got: %s",
							goroutineID, k, expectedUser.ID, user.ID)
					}

					if user.Email != expectedUser.Email {
						t.Errorf("Goroutine %d, user %d: Email corruption. Expected: %s, Got: %s",
							goroutineID, k, expectedUser.Email, user.Email)
					}

					// Check nested data
					if len(user.Profile.PhoneNumbers) != len(expectedUser.Profile.PhoneNumbers) {
						t.Errorf("Goroutine %d, user %d: PhoneNumbers length corruption. Expected: %d, Got: %d",
							goroutineID, k, len(expectedUser.Profile.PhoneNumbers), len(user.Profile.PhoneNumbers))
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			t.Errorf("Slice race condition test failed with error: %v", err)
		}
	}
}

// TestJsonRaceConditionPointerFields tests race conditions with pointer fields
func TestJsonRaceConditionPointerFields(t *testing.T) {
	const numGoroutines = 25
	const numOperations = 40

	// Create test data with pointer fields
	testAddress := GenerateAddressWithCoordinates()
	expectedLat := testAddress.Coordinates.Latitude
	expectedLng := testAddress.Coordinates.Longitude

	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines*numOperations)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Encode with pointer field
				jsonData, err := Convert(&testAddress).JsonEncode()
				if err != nil {
					errorChan <- err
					return
				}

				// Decode with pointer field
				var decodedAddress ComplexAddress
				err = Convert(string(jsonData)).JsonDecode(&decodedAddress)
				if err != nil {
					errorChan <- err
					return
				}

				// Verify pointer field integrity
				if decodedAddress.Coordinates == nil {
					t.Errorf("Goroutine %d: Coordinates pointer corrupted - nil", goroutineID)
					continue
				}

				if decodedAddress.Coordinates.Latitude != expectedLat {
					t.Errorf("Goroutine %d: Latitude corruption. Expected: %f, Got: %f",
						goroutineID, expectedLat, decodedAddress.Coordinates.Latitude)
				}

				if decodedAddress.Coordinates.Longitude != expectedLng {
					t.Errorf("Goroutine %d: Longitude corruption. Expected: %f, Got: %f",
						goroutineID, expectedLng, decodedAddress.Coordinates.Longitude)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			t.Errorf("Pointer field race condition test failed with error: %v", err)
		}
	}
}
