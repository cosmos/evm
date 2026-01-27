import hashlib
from typing import Optional

import pytest
from eth_account import Account
from eth_account.messages import encode_defunct
from eth_utils import to_hex
from hexbytes import HexBytes
from py_ecc.bls12_381 import G1, G2, add, multiply
from py_ecc.fields import FQ

pytestmark = pytest.mark.asyncio


# Berlin precompile addresses
PRECOMPILE_ECRECOVER = "0x0000000000000000000000000000000000000001"
PRECOMPILE_SHA256 = "0x0000000000000000000000000000000000000002"
PRECOMPILE_RIPEMD160 = "0x0000000000000000000000000000000000000003"
PRECOMPILE_DATACOPY = "0x0000000000000000000000000000000000000004"
PRECOMPILE_BIGMODEXP = "0x0000000000000000000000000000000000000005"
PRECOMPILE_BN256ADD = "0x0000000000000000000000000000000000000006"
PRECOMPILE_BN256SCALARMUL = "0x0000000000000000000000000000000000000007"
PRECOMPILE_BN256PAIRING = "0x0000000000000000000000000000000000000008"
PRECOMPILE_BLAKE2F = "0x0000000000000000000000000000000000000009"
# Cancun precompile addresses
PRECOMPILE_KZG_POINT_EVALUATION = "0x000000000000000000000000000000000000000A"
# Prague precompile addresses
PRECOMPILE_BLS12381_G1_ADD = "0x000000000000000000000000000000000000000b"
PRECOMPILE_BLS12381_G1_MULTIEXP = "0x000000000000000000000000000000000000000C"
PRECOMPILE_BLS12381_G2_ADD = "0x000000000000000000000000000000000000000d"
PRECOMPILE_BLS12381_G2_MULTIEXP = "0x000000000000000000000000000000000000000E"
PRECOMPILE_BLS12381_PAIRING = "0x000000000000000000000000000000000000000F"
PRECOMPILE_BLS12381_MAP_G1 = "0x0000000000000000000000000000000000000010"
PRECOMPILE_BLS12381_MAP_G2 = "0x0000000000000000000000000000000000000011"


async def test_ecrecover(evm):
    w3 = evm.async_w3
    account = Account.create()
    message = b"hello world"
    signable_message = encode_defunct(message)
    signed_message = account.sign_message(signable_message)
    message_hash = signed_message.message_hash
    v = signed_message.v
    r = signed_message.r
    s = signed_message.s
    # Prepare input: hash(32) + v(32) + r(32) + s(32)
    input_data = (
        message_hash
        + v.to_bytes(32, "big")
        + r.to_bytes(32, "big")
        + s.to_bytes(32, "big")
    )
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_ECRECOVER,
            "data": to_hex(input_data),
        }
    )
    # The result should be the address (last 20 bytes, padded to 32)
    recovered_address = "0x" + result[-20:].hex()
    expected_address = account.address.lower()

    print(f"ecrecover result: {result.hex()}")
    print(f"recovered address: {recovered_address}")
    print(f"expected address: {expected_address}")

    assert recovered_address == expected_address
    assert len(result) == 32


async def test_sha256(evm):
    w3 = evm.async_w3
    test_data = b"hello world"
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_SHA256,
            "data": to_hex(test_data),
        }
    )
    expected = hashlib.sha256(test_data).digest()
    assert result == expected
    print(f"SHA-256 test passed: {result.hex()}")


async def test_ripemd160(evm):
    w3 = evm.async_w3
    test_data = b"hello world"
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_RIPEMD160,
            "data": to_hex(test_data),
        }
    )
    # RIPEMD-160 returns 20 bytes, left-padded to 32 bytes
    assert len(result) == 32
    print(f"RIPEMD-160 test passed: {result.hex()}")


async def test_identity(evm):
    w3 = evm.async_w3
    test_data = b"hello world test data"
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_DATACOPY,
            "data": to_hex(test_data),
        }
    )
    assert result == test_data
    print(f"Identity test passed: {result.hex()}")


async def test_bigmodexp(evm):
    w3 = evm.async_w3
    # Test 2^2 mod 3 = 1
    base_len = (1).to_bytes(32, "big")  # base length = 1
    exp_len = (1).to_bytes(32, "big")  # exponent length = 1
    mod_len = (1).to_bytes(32, "big")  # modulus length = 1
    base = (2).to_bytes(1, "big")  # base = 2
    exp = (2).to_bytes(1, "big")  # exponent = 2
    mod = (3).to_bytes(1, "big")  # modulus = 3
    input_data = base_len + exp_len + mod_len + base + exp + mod
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_BIGMODEXP,
            "data": to_hex(input_data),
        }
    )
    # 2^2 mod 3 = 4 mod 3 = 1
    expected = (1).to_bytes(1, "big")
    assert result == expected
    print(f"BigModExp test passed: {result.hex()}")


async def test_bn256add(evm):
    w3 = evm.async_w3
    # Test adding two valid points on the curve
    # Using generator point (1, 2) + point at infinity
    p1_x = (1).to_bytes(32, "big")
    p1_y = (2).to_bytes(32, "big")
    p2_x = (0).to_bytes(32, "big")  # Point at infinity
    p2_y = (0).to_bytes(32, "big")
    input_data = p1_x + p1_y + p2_x + p2_y
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_BN256ADD,
            "data": to_hex(input_data),
        }
    )
    assert len(result) == 64  # Should return 64 bytes (two 32-byte coordinates)
    print(f"bn256Add test passed: {result.hex()}")


async def test_bn256scalarmul(evm):
    w3 = evm.async_w3
    # Multiply generator point by scalar 1
    p_x = (1).to_bytes(32, "big")
    p_y = (2).to_bytes(32, "big")
    scalar = (1).to_bytes(32, "big")
    input_data = p_x + p_y + scalar
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_BN256SCALARMUL,
            "data": to_hex(input_data),
        }
    )
    assert len(result) == 64
    print(f"bn256ScalarMul test passed: {result.hex()}")


async def test_bn256pairing(evm):
    w3 = evm.async_w3
    # Empty input should return true (1)
    result = await w3.eth.call({"to": PRECOMPILE_BN256PAIRING, "data": "0x"})
    # Empty pairing should return 1 (true)
    expected = (1).to_bytes(32, "big")
    assert result == expected
    print(f"bn256Pairing test passed: {result.hex()}")


async def test_blake2f(evm):
    w3 = evm.async_w3
    # Minimal test with 1 round
    rounds = (1).to_bytes(4, "big")
    h = b"\x00" * 64  # 64 bytes of state
    m = b"\x00" * 128  # 128 bytes of message
    t = b"\x00" * 16  # 16 bytes of offset counters
    final_flag = b"\x01"  # 1 byte final flag
    input_data = rounds + h + m + t + final_flag
    result = await w3.eth.call(
        {
            "to": PRECOMPILE_BLAKE2F,
            "data": to_hex(input_data),
        }
    )
    assert len(result) == 64  # Should return 64 bytes
    print(f"BLAKE2F test passed: {result.hex()}")


async def test_all_berlin_precompiles_exist(evm):
    w3 = evm.async_w3
    berlin_precompiles = [
        PRECOMPILE_ECRECOVER,
        PRECOMPILE_SHA256,
        PRECOMPILE_RIPEMD160,
        PRECOMPILE_DATACOPY,
        PRECOMPILE_BIGMODEXP,
        PRECOMPILE_BN256ADD,
        PRECOMPILE_BN256SCALARMUL,
        PRECOMPILE_BN256PAIRING,
        PRECOMPILE_BLAKE2F,
    ]
    for address in berlin_precompiles:
        # Check if precompile exists by getting code (should be empty)
        code = await w3.eth.get_code(address)
        print(f"{address}: code length {len(code)}")
        # Precompiles should have empty code but still be callable
        assert len(code) == 0, f"{address} should have empty code"


async def test_precompile_gas_costs_berlin(evm):
    w3 = evm.async_w3
    # Test simple inputs to verify gas costs
    test_cases = [
        (PRECOMPILE_ECRECOVER, "0x" + "00" * 128, 3000),  # ECRecover base cost
        (PRECOMPILE_SHA256, "0x" + "00" * 32, 60),  # SHA256 base cost
        (PRECOMPILE_RIPEMD160, "0x" + "00" * 32, 600),  # RIPEMD160 base cost
        (PRECOMPILE_DATACOPY, "0x" + "00" * 32, 15),  # Identity base cost
        (PRECOMPILE_BN256ADD, "0x" + "00" * 128, 150),  # BN256Add cost
        (PRECOMPILE_BN256SCALARMUL, "0x" + "00" * 96, 6000),  # BN256ScalarMul cost
    ]
    for address, data, expected_min_gas in test_cases:
        gas_estimate = await w3.eth.estimate_gas(
            {
                "to": address,
                "data": data,
            }
        )
        print(
            f"{address}: estimated gas {gas_estimate}, expected min {expected_min_gas}"
        )
        assert gas_estimate >= expected_min_gas, f"Gas estimate too low for {address}"


class Spec:
    # BLS12-381 field modulus
    BLS_MODULUS = 0x73EDA753299D7D483339D80809A1D80553BDA402FFFE5BFEFFFFFFFF00000001
    INF_POINT = b"\xc0" + b"\x00" * 47

    @staticmethod
    def pad_fq(x: FQ) -> bytes:
        # py_ecc FQ elements are integers mod prime, 48 bytes big-endian native size
        # Ethereum expects 64 bytes left-padded with zeros
        as_bytes = x.n.to_bytes(48, "big")
        return as_bytes.rjust(64, b"\x00")

    @classmethod
    def point_to_bytes(cls, point) -> bytes:
        if point is None:
            # Represent point at infinity as zeros (128 bytes)
            return b"\x00" * 128
        x, y = point
        return cls.pad_fq(x) + cls.pad_fq(y)

    @classmethod
    def g2_point_to_bytes(cls, point) -> bytes:
        """Convert G2 point to 256 bytes."""
        if point is None:
            # Represent point at infinity as zeros (256 bytes)
            return b"\x00" * 256
        x, y = point
        # G2 points have Fp2 coordinates: x = (x.c0, x.c1), y = (y.c0, y.c1)
        # Each component is an FQ element that needs 64-byte padding
        x_c0_bytes = cls.pad_fq(x.coeffs[0])  # 64 bytes
        x_c1_bytes = cls.pad_fq(x.coeffs[1])  # 64 bytes
        y_c0_bytes = cls.pad_fq(y.coeffs[0])  # 64 bytes
        y_c1_bytes = cls.pad_fq(y.coeffs[1])  # 64 bytes
        return x_c0_bytes + x_c1_bytes + y_c0_bytes + y_c1_bytes


# Identity element (point at infinity)
G1_IDENTITY = b"\x00" * 128
G2_IDENTITY = b"\x00" * 256
G1_GENERATOR = Spec.point_to_bytes(G1)
G2_GENERATOR = Spec.g2_point_to_bytes(G2)

# Double the generator point: 2*G1
G1_GENERATOR_DOUBLE_POINT = add(G1, G1)
G1_GENERATOR_DOUBLE = Spec.point_to_bytes(G1_GENERATOR_DOUBLE_POINT)

# Example arbitrary test point (multiply G1 by 7)
TEST_POINT_1_POINT = multiply(G1, 7)
TEST_POINT_1 = Spec.point_to_bytes(TEST_POINT_1_POINT)


def kzg_to_versioned_hash(kzg_commitment: bytes) -> bytes:
    """Convert KZG commitment to versioned hash."""
    hash_result = hashlib.sha256(kzg_commitment).digest()
    # Add version byte (0x01 for KZG)
    return bytes([0x01]) + hash_result[1:]


async def format_precompile_input(
    versioned_hash: Optional[bytes],
    z: int,
    y: int,
    kzg_commitment: bytes,
    kzg_proof: bytes,
) -> bytes:
    """Format the input for the point evaluation precompile (192 bytes total)."""
    z_bytes = z.to_bytes(32, "big")
    y_bytes = y.to_bytes(32, "big")
    if versioned_hash is None:
        versioned_hash = kzg_to_versioned_hash(kzg_commitment)
    return versioned_hash + z_bytes + y_bytes + kzg_commitment + kzg_proof


async def test_kzg_point_evaluation(evm):
    w3 = evm.async_w3
    # Use a valid input (infinity point)
    input = await format_precompile_input(
        versioned_hash=None,
        z=Spec.BLS_MODULUS - 1,
        y=0,
        kzg_commitment=Spec.INF_POINT,
        kzg_proof=Spec.INF_POINT,
    )
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_KZG_POINT_EVALUATION,
            "data": to_hex(input),
            "gas": 150000,
        }
    )
    prefix_bytes = b"\x00" * 30 + b"\x10\x00"
    modulus_bytes = Spec.BLS_MODULUS.to_bytes(32, "big")
    assert res == prefix_bytes + modulus_bytes


@pytest.mark.parametrize(
    "point_a,point_b,expected",
    [
        # Identity element tests
        (G1_IDENTITY, G1_IDENTITY, G1_IDENTITY),
        (G1_GENERATOR, G1_IDENTITY, G1_GENERATOR),
        (G1_IDENTITY, G1_GENERATOR, G1_GENERATOR),
        # Valid point addition tests
        (G1_GENERATOR, G1_GENERATOR, G1_GENERATOR_DOUBLE),
        (TEST_POINT_1, G1_IDENTITY, TEST_POINT_1),
    ],
    ids=[
        "identity_plus_identity",
        "generator_plus_identity",
        "identity_plus_generator",
        "generator_plus_generator",
        "test_point_plus_identity",
    ],
)
async def test_bls12381_g1_add(evm, point_a, point_b: bytes, expected):
    # Verify input points are correct length
    if len(point_a) != 128 or len(point_b) != 128:
        raise ValueError("Each G1 point must be exactly 128 bytes")
    input_data = point_a + point_b
    w3 = evm.async_w3
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_BLS12381_G1_ADD,
            "data": "0x" + input_data.hex(),
            "gas": 30000,
        }
    )
    assert len(res) == 128
    assert expected == res


async def test_bls12381_g1_multiexp(evm):
    w3 = evm.async_w3
    # Test case 1: Single point multiplication - G * 2 = 2G
    # Input format: point (128 bytes) + scalar (32 bytes) = 160 bytes per pair
    point1 = G1_GENERATOR
    scalar1 = (2).to_bytes(32, "big")  # Multiply by 2
    input_data = point1 + scalar1
    # Verify input points are correct length
    assert len(point1) == 128, f"Point should be 128 bytes, got {len(point1)}"
    assert len(scalar1) == 32, f"Scalar should be 32 bytes, got {len(scalar1)}"
    assert (
        len(input_data) == 160
    ), f"Single multiexp input should be 160 bytes, got {len(input_data)}"
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_BLS12381_G1_MULTIEXP,
            "data": "0x" + input_data.hex(),
            "gas": 50000,
        }
    )
    assert len(res) == 128, f"Result should be 128 bytes, got {len(res)}"
    assert res == G1_GENERATOR_DOUBLE, "G * 2 should equal 2G"


async def test_bls12381_g2_add(evm):
    w3 = evm.async_w3
    # G2 points are 256 bytes each (128 bytes for X + 128 bytes for Y)
    # Each coordinate (X, Y) is 128 bytes because G2 is over Fp2 (two Fp elements)
    # Verify the generator point is exactly 256 bytes
    assert (
        len(G2_GENERATOR) == 256
    ), f"G2 generator should be 256 bytes, got {len(G2_GENERATOR)}"

    async def call(input):
        return await w3.eth.call(
            {
                "to": PRECOMPILE_BLS12381_G2_ADD,
                "data": f"0x{input.hex()}",
                "gas": 50000,
            }
        )

    # Test case 1: Identity + Identity = Identity
    input = G2_IDENTITY + G2_IDENTITY
    assert len(input) == 512, f"G2 Add input should be 512 bytes, got {len(input)}"
    res = await call(input)
    assert len(res) == 256, f"G2 result should be 256 bytes, got {len(res)}"
    assert res == G2_IDENTITY, "Identity + Identity should equal Identity"

    # Test case 2: Generator + Identity = Generator
    input = G2_GENERATOR + G2_IDENTITY
    assert len(input) == 512, f"G2 Add input should be 512 bytes, got {len(input)}"
    res = await call(input)
    assert len(res) == 256, f"G2 result should be 256 bytes, got {len(res)}"
    assert res == G2_GENERATOR, "Generator + Identity should equal Generator"

    # Test case 3: Identity + Generator = Generator
    input = G2_IDENTITY + G2_GENERATOR
    assert len(input) == 512, f"G2 Add input should be 512 bytes, got {len(input)}"
    res = await call(input)
    assert len(res) == 256, f"G2 result should be 256 bytes, got {len(res)}"
    assert res == G2_GENERATOR, "Identity + Generator should equal Generator"

    # Test case 4: Generator + Generator = 2*Generator (point doubling)
    input = G2_GENERATOR + G2_GENERATOR
    assert len(input) == 512, f"G2 Add input should be 512 bytes, got {len(input)}"
    res = await call(input)
    assert len(res) == 256, f"G2 result should be 256 bytes, got {len(res)}"
    assert res != G2_GENERATOR, "Generator + Generator should not equal Generator"
    assert res != G2_IDENTITY, "Generator + Generator should not equal Identity"


@pytest.mark.parametrize(
    "points,scalars,expected",
    [
        # Single point multiplication
        ([G2_GENERATOR], [1], G2_GENERATOR),  # G2 * 1 = G2
        ([G2_IDENTITY], [5], G2_IDENTITY),  # Identity * 5 = Identity
        ([G2_GENERATOR], [0], G2_IDENTITY),  # G2 * 0 = Identity
        # Multiple points with zero scalars
        ([G2_GENERATOR, G2_GENERATOR], [0, 0], G2_IDENTITY),  # G2*0 + G2*0 = Identity
        # Identity point multiplication (should always result in identity)
        (
            [G2_IDENTITY, G2_IDENTITY],
            [100, 200],
            G2_IDENTITY,
        ),  # Identity*100 + Identity*200 = Identity
    ],
    ids=[
        "g2_times_1",
        "identity_times_5",
        "g2_times_0",
        "zero_scalars",
        "identity_multiplication",
    ],
)
async def test_bls12381_g2_multiexp(evm, points, scalars, expected):
    w3 = evm.async_w3
    assert len(points) == len(scalars), "Number of points must equal number of scalars"
    # Build input data: point1 + scalar1 + point2 + scalar2 + ...
    input_data = b""
    for point, scalar in zip(points, scalars):
        assert len(point) == 256, f"Each G2 point must be 256 bytes, got {len(point)}"
        scalar_bytes = scalar.to_bytes(32, "big")
        input_data += point + scalar_bytes

    expected_input_length = len(points) * 288  # 288 bytes per (point + scalar) pair
    assert (
        len(input_data) == expected_input_length
    ), f"Input should be {expected_input_length} bytes, got {len(input_data)}"
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_BLS12381_G2_MULTIEXP,
            "data": "0x" + input_data.hex(),
            "gas": 100000,
        }
    )
    assert len(res) == 256, f"G2 result should be 256 bytes, got {len(res)}"
    assert res == expected, f"Expected {expected.hex()}, got {res.hex()}"


@pytest.mark.parametrize(
    "pairs",
    [
        # Single pair with identities (should be true)
        ([(G1_IDENTITY, G2_GENERATOR)]),
        ([(G1_GENERATOR, G2_IDENTITY)]),
        ([(G1_IDENTITY, G2_IDENTITY)]),
        # Multiple pairs that should result in true
        ([(G1_IDENTITY, G2_GENERATOR), (G1_GENERATOR, G2_IDENTITY)]),
        ([(G1_IDENTITY, G2_IDENTITY), (G1_IDENTITY, G2_IDENTITY)]),
    ],
    ids=[
        "g1_identity_g2_gen",
        "g1_gen_g2_identity",
        "double_identity",
        "mixed_identities",
        "all_identities",
    ],
)
async def test_bls12381_pairing(evm, pairs):
    w3 = evm.async_w3
    # Build input data: g1_point1 + g2_point1 + g1_point2 + g2_point2 + ...
    input_data = b""
    for g1_point, g2_point in pairs:
        assert len(g1_point) == 128, f"G1 point must be 128 bytes, got {len(g1_point)}"
        assert len(g2_point) == 256, f"G2 point must be 256 bytes, got {len(g2_point)}"
        input_data += g1_point + g2_point

    expected_input_length = len(pairs) * 384  # 384 bytes per (G1 + G2) pair
    assert (
        len(input_data) == expected_input_length
    ), f"Input should be {expected_input_length} bytes, got {len(input_data)}"
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_BLS12381_PAIRING,
            "data": "0x" + input_data.hex(),
            "gas": 200000 + len(pairs) * 100000,  # More gas for more pairs
        }
    )
    assert len(res) == 32, f"Pairing result should be 32 bytes, got {len(res)}"
    expected_bytes = b"\x00" * 31 + b"\x01"
    assert res == expected_bytes, f"Expected true (1), got {res.hex()}"


@pytest.mark.parametrize(
    "field",
    [
        0,
        1,
        2,
        42,
        2**64 - 1,
        Spec.BLS_MODULUS // 2,
        Spec.BLS_MODULUS - 1,
    ],
    ids=[
        "zero",
        "one",
        "two",
        "forty_two",
        "large_number",
        "half_modulus",
        "max_field_element",
    ],
)
async def test_bls12381_map_g1(evm, field):
    w3 = evm.async_w3
    input = field.to_bytes(64, "big")
    assert len(input) == 64, f"input should be 64 bytes, got {len(input)}"
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_BLS12381_MAP_G1,
            "data": f"0x{input.hex()}",
            "gas": 50000,
        }
    )
    assert len(res) == 128, f"G1 map result should be 128 bytes, got {len(res)}"


@pytest.mark.parametrize(
    "field_pair",
    [
        (0, 0),
        (1, 0),
        (0, 1),
        (1, 1),
        (42, 123),
        (2**64 - 1, 2**32 - 1),
        (Spec.BLS_MODULUS // 2, Spec.BLS_MODULUS // 3),
        (Spec.BLS_MODULUS - 1, Spec.BLS_MODULUS - 2),
    ],
    ids=[
        "zero_zero",
        "one_zero",
        "zero_one",
        "one_one",
        "forty_two_oneTwoThree",
        "large_numbers",
        "half_third_modulus",
        "max_field_elements",
    ],
)
async def test_bls12381_map_g2(evm, field_pair):
    w3 = evm.async_w3
    field1, field2 = field_pair
    # G2 map takes two field elements (128 bytes total: 64 bytes each)
    # This represents an Fp2 element: field1 + field2 * i
    data1 = field1.to_bytes(64, "big")
    data2 = field2.to_bytes(64, "big")
    input = data1 + data2
    assert len(input) == 128, f"G2 map input should be 128 bytes, got {len(input)}"
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_BLS12381_MAP_G2,
            "data": "0x" + input.hex(),
            "gas": 75000,
        }
    )
    assert len(res) == 256, f"G2 map result should be 256 bytes, got {len(res)}"


async def test_move_precompile_to(evm):
    w3 = evm.async_w3
    moved = w3.to_checksum_address("0x00000000000000000000000000000000deadbeef")
    input = 1
    code = "0x60003560010160005260206000f3"  # bytecode: adds one to input
    res = await w3.eth.call(
        {
            "to": PRECOMPILE_ECRECOVER,
            "data": input.to_bytes(32, byteorder="big"),
        },
        "latest",
        {
            PRECOMPILE_ECRECOVER: {
                "code": code,
                "movePrecompileToAddress": moved,
            },
        },
    )
    output = 2
    assert res == output.to_bytes(32, byteorder="big")

    input = "0x82f3df49d3645876de6313df2bbe9fbce593f21341a7b03acdb9423bc171fcc9000000000000000000000000000000000000000000000000000000000000001cba13918f50da910f2d55a7ea64cf716ba31dad91856f45908dde900530377d8a112d60f36900d18eb8f9d3b4f85a697b545085614509e3520e4b762e35d0d6bd"  # noqa: E501
    res = await w3.eth.call(
        {
            "to": moved,
            "data": HexBytes(input),
        },
        "latest",
        {
            PRECOMPILE_ECRECOVER: {
                "code": code,
                "movePrecompileToAddress": moved,
            },
        },
    )
    output = "0x000000000000000000000000c6e93f4c1920eaeaa1e699f76a7a8c18e3056074"
    assert res == HexBytes(output)
