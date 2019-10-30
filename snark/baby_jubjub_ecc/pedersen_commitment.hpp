/*    
    copyright 2018 to the baby_jubjub_ecc Authors

    This file is part of baby_jubjub_ecc.

    baby_jubjub_ecc is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    baby_jubjub_ecc is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with baby_jubjub_ecc.  If not, see <https://www.gnu.org/licenses/>.
*/


#include <cassert>
#include <memory>

#include <libsnark/gadgetlib1/gadget.hpp>

#ifndef BABY_JUBJUB_CPP
#define BABY_JUBJUB_CPP
#endif

using namespace libsnark;

template<typename FieldT>
class  pedersen_commitment : public gadget<FieldT> {

private:
    pb_variable<FieldT> a;
    pb_variable<FieldT> d;


    //input variables
    pb_variable<FieldT> base_x;
    pb_variable<FieldT> base_y;
    pb_variable_array<FieldT> A;
    pb_variable<FieldT> H_x;
    pb_variable<FieldT> H_y;
    pb_variable_array<FieldT> m;
    pb_variable_array<FieldT> r;
    pb_variable<FieldT> r_x;
    pb_variable<FieldT> r_y;
    pb_variable<FieldT> s_x;
    pb_variable<FieldT> s_y;


    //intermeidate variables

    pb_variable<FieldT> res_x;
    pb_variable<FieldT> res_y;

    pb_variable_array<FieldT> lhs_x;

    pb_variable_array<FieldT> lhs_y;
    pb_variable_array<FieldT> rhs_mul_x;
    pb_variable_array<FieldT> rhs_mul_y;

    pb_variable_array<FieldT> rhs_x;
    pb_variable_array<FieldT> rhs_y;

    std::shared_ptr<isOnCurve<FieldT>> jubjub_isOnCurve1;
    std::shared_ptr<isOnCurve<FieldT>> jubjub_isOnCurve2;

    std::shared_ptr<pointAddition<FieldT>> jubjub_pointAddition;
    std::shared_ptr<pointMultiplication<FieldT>> jubjub_pointMultiplication_lhs;
    std::shared_ptr<pointMultiplication<FieldT>> jubjub_pointMultiplication_rhs;
public:

    pb_variable<FieldT> commitment_x;
    pb_variable<FieldT> commitment_y;

     pedersen_commitment(protoboard<FieldT> &pb,
                   //const pb_linear_combination_array<FieldT> &bits,
                   const pb_variable<FieldT> &commitment_x, const pb_variable<FieldT> &commitment_y,
                   const pb_variable_array<FieldT> &m, const pb_variable_array<FieldT> &r
                   );

    void generate_r1cs_constraints();
    void generate_r1cs_witness();
    pb_variable<FieldT> get_res_x();
    pb_variable<FieldT> get_res_y();
};

#include <pedersen_commitment.cpp>
