module distconssim

replace centralsim => ../centralsim

replace comm_vector => ../comm_vector

replace comm => ../comm

require (
	centralsim v0.0.0-00010101000000-000000000000
	comm v0.0.0-00010101000000-000000000000 // indirect
	comm_vector v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
)

go 1.15
